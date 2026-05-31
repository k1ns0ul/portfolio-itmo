package grpc

import (
	"context"
	"fmt"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ClientPool struct {
	mu    sync.Mutex
	conns map[string]*grpc.ClientConn
}

func NewClientPool() *ClientPool {
	return &ClientPool{conns: make(map[string]*grpc.ClientConn)}
}

func (p *ClientPool) conn(addr string) (*grpc.ClientConn, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if c, ok := p.conns[addr]; ok {
		return c, nil
	}
	c, err := grpc.Dial(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.CallContentSubtype(codecName)),
	)
	if err != nil {
		return nil, fmt.Errorf("dial leader grpc %s: %w", addr, err)
	}
	p.conns[addr] = c
	return c, nil
}

func (p *ClientPool) Forward(ctx context.Context, addr string, req *ForwardRequest) (*ForwardResponse, error) {
	conn, err := p.conn(addr)
	if err != nil {
		return nil, err
	}
	out := new(ForwardResponse)
	if err := conn.Invoke(ctx, "/"+ForwardServiceName+"/Forward", req, out); err != nil {
		return nil, fmt.Errorf("forward to %s: %w", addr, err)
	}
	return out, nil
}

func (p *ClientPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for addr, c := range p.conns {
		c.Close()
		delete(p.conns, addr)
	}
}
