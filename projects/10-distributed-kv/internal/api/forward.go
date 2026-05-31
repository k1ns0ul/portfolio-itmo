package api

import (
	"context"
	"fmt"

	dkvgrpc "github.com/andrey/distributed-kv/internal/grpc"
)

type Forwarder struct {
	pool    *dkvgrpc.ClientPool
	leader  func() string
	resolve func(raftAddr string) string
}

func NewForwarder(pool *dkvgrpc.ClientPool, leader func() string, resolve func(string) string) *Forwarder {
	if resolve == nil {
		resolve = func(addr string) string { return addr }
	}
	return &Forwarder{pool: pool, leader: leader, resolve: resolve}
}

func (f *Forwarder) ForwardToLeader(ctx context.Context, method, path string, body []byte) (int, []byte, error) {
	leaderRaft := f.leader()
	if leaderRaft == "" {
		return 503, nil, fmt.Errorf("no known leader, cluster may be electing")
	}
	target := f.resolve(leaderRaft)
	if target == "" {
		return 503, nil, fmt.Errorf("cannot resolve grpc endpoint for leader %s", leaderRaft)
	}

	req := &dkvgrpc.ForwardRequest{Method: method, Path: path, Body: body}
	resp, err := f.pool.Forward(ctx, target, req)
	if err != nil {
		retryRaft := f.leader()
		retryTarget := f.resolve(retryRaft)
		if retryTarget == "" || retryTarget == target {
			return 503, nil, fmt.Errorf("forward failed and leader unchanged: %w", err)
		}
		resp, err = f.pool.Forward(ctx, retryTarget, req)
		if err != nil {
			return 503, nil, fmt.Errorf("forward retry to %s failed: %w", retryTarget, err)
		}
	}
	return int(resp.StatusCode), resp.Body, nil
}
