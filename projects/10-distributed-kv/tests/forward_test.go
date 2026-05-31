package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/andrey/distributed-kv/internal/api"
	dkvgrpc "github.com/andrey/distributed-kv/internal/grpc"
)

type fakeForwardServer struct {
	received *dkvgrpc.ForwardRequest
}

func (f *fakeForwardServer) Forward(_ context.Context, req *dkvgrpc.ForwardRequest) (*dkvgrpc.ForwardResponse, error) {
	f.received = req
	return &dkvgrpc.ForwardResponse{StatusCode: http.StatusOK, Body: []byte(`{"status":"stored"}`)}, nil
}

type recordingExecutor struct {
	method string
	path   string
	body   []byte
}

func (r *recordingExecutor) Execute(_ context.Context, method, path string, body []byte) (int, []byte) {
	r.method = method
	r.path = path
	r.body = body
	return http.StatusOK, []byte(`{"status":"stored"}`)
}

func TestForwarderProxiesToLeader(t *testing.T) {
	exec := &recordingExecutor{}
	srv := dkvgrpc.NewServer(exec)
	lis := mustListen(t)
	go srv.Serve(lis.Addr().String())
	defer srv.Stop()
	lis.Close()

	pool := dkvgrpc.NewClientPool()
	defer pool.Close()

	forwarder := api.NewForwarder(pool, func() string { return "" }, nil)
	status, _, err := forwarder.ForwardToLeader(context.Background(), http.MethodPut, "/api/v1/kv/x", []byte("v"))
	if err == nil {
		t.Fatal("expected error when leader is unknown")
	}
	if status != http.StatusServiceUnavailable {
		t.Fatalf("got status %d want 503", status)
	}
}

func TestExecuteRoundTripThroughServer(t *testing.T) {
	exec := &recordingExecutor{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status, body := exec.Execute(r.Context(), r.Method, r.URL.Path, nil)
		w.WriteHeader(status)
		w.Write(body)
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/kv/demo")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got %d want 200", resp.StatusCode)
	}
	if exec.path != "/api/v1/kv/demo" {
		t.Fatalf("executor saw path %q", exec.path)
	}
}
