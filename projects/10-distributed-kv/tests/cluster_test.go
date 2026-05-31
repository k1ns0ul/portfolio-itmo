package tests

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	hraft "github.com/hashicorp/raft"

	"github.com/andrey/distributed-kv/internal/api"
	dkvgrpc "github.com/andrey/distributed-kv/internal/grpc"
	dkvraft "github.com/andrey/distributed-kv/internal/raft"
	"github.com/andrey/distributed-kv/internal/store"
)

func mustListen(t *testing.T) net.Listener {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	return lis
}

func freeAddr(t *testing.T) string {
	t.Helper()
	lis := mustListen(t)
	addr := lis.Addr().String()
	lis.Close()
	return addr
}

type testNode struct {
	id       string
	raftAddr string
	grpcAddr string
	httpAddr string
	node     *dkvraft.RaftNode
	store    *store.KVStore
	server   *api.Server
	httpSrv  *http.Server
	grpcSrv  *dkvgrpc.Server
}

func startCluster(t *testing.T, n int) []*testNode {
	t.Helper()
	nodes := make([]*testNode, n)
	servers := make([]hraft.Server, n)
	registry := make(map[string]string)

	for i := 0; i < n; i++ {
		id := fmt.Sprintf("node%d", i+1)
		raftAddr := freeAddr(t)
		grpcAddr := freeAddr(t)
		nodes[i] = &testNode{id: id, raftAddr: raftAddr, grpcAddr: grpcAddr}
		registry[raftAddr] = grpcAddr
		servers[i] = hraft.Server{ID: hraft.ServerID(id), Address: hraft.ServerAddress(raftAddr)}
	}

	resolve := func(raftAddr string) string { return registry[raftAddr] }

	for i, tn := range nodes {
		kv := store.NewKVStore()
		node, err := dkvraft.NewRaftNode(dkvraft.Config{
			ID:       tn.id,
			RaftAddr: tn.raftAddr,
			DataDir:  filepath.Join(t.TempDir(), tn.id),
		}, kv)
		if err != nil {
			t.Fatalf("create node %s: %v", tn.id, err)
		}
		if i == 0 {
			if err := node.Bootstrap(servers); err != nil {
				t.Fatalf("bootstrap: %v", err)
			}
		}

		pool := dkvgrpc.NewClientPool()
		t.Cleanup(pool.Close)
		forwarder := api.NewForwarder(pool, node.Leader, resolve)
		server := api.NewServer(api.Deps{Node: node, Store: kv, Forwarder: forwarder, Log: testLogger()})

		grpcSrv := dkvgrpc.NewServer(server)
		go grpcSrv.Serve(tn.grpcAddr)

		httpAddr := freeAddr(t)
		httpSrv := &http.Server{Addr: httpAddr, Handler: server.Handler()}
		ln, err := net.Listen("tcp", httpAddr)
		if err != nil {
			t.Fatalf("http listen: %v", err)
		}
		go httpSrv.Serve(ln)

		tn.node = node
		tn.store = kv
		tn.server = server
		tn.httpAddr = httpAddr
		tn.httpSrv = httpSrv
		tn.grpcSrv = grpcSrv
	}

	t.Cleanup(func() {
		for _, tn := range nodes {
			if tn.httpSrv != nil {
				tn.httpSrv.Close()
			}
			if tn.grpcSrv != nil {
				tn.grpcSrv.Stop()
			}
			if tn.node != nil {
				tn.node.Shutdown()
			}
		}
	})

	return nodes
}

func waitForLeader(t *testing.T, nodes []*testNode) *testNode {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		for _, tn := range nodes {
			if tn.node.IsLeader() {
				return tn
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("no leader elected within timeout")
	return nil
}

func httpPut(t *testing.T, addr, key string, value []byte) int {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPut, "http://"+addr+"/api/v1/kv/"+key, bytes.NewReader(value))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}

func httpGet(t *testing.T, addr, key, query string) (int, []byte) {
	t.Helper()
	url := "http://" + addr + "/api/v1/kv/" + key
	if query != "" {
		url += "?" + query
	}
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, body
}

func TestClusterReplicationAndForward(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping cluster test in short mode")
	}
	nodes := startCluster(t, 3)
	leader := waitForLeader(t, nodes)
	time.Sleep(500 * time.Millisecond)

	if code := httpPut(t, leader.httpAddr, "color", []byte("blue")); code != http.StatusOK {
		t.Fatalf("put on leader: got %d", code)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	found := false
	for !found {
		select {
		case <-ctx.Done():
			t.Fatal("value did not replicate in time")
		default:
		}
		for _, tn := range nodes {
			if v, ok := tn.store.Get("color"); ok && string(v) == "blue" {
				found = true
			}
		}
		if !found {
			time.Sleep(100 * time.Millisecond)
		}
	}

	var follower *testNode
	for _, tn := range nodes {
		if !tn.node.IsLeader() {
			follower = tn
			break
		}
	}
	if follower == nil {
		t.Fatal("no follower found")
	}

	if code := httpPut(t, follower.httpAddr, "shape", []byte("circle")); code != http.StatusOK {
		t.Fatalf("forwarded put failed: got %d", code)
	}

	code, _ := httpGet(t, follower.httpAddr, "shape", "consistent=true")
	if code != http.StatusServiceUnavailable {
		t.Fatalf("consistent read on follower: got %d want 503", code)
	}
}
