package raft

import (
	"fmt"
	"net"
	"time"

	"github.com/hashicorp/raft"
)

const (
	transportMaxPool = 5
	transportTimeout = 10 * time.Second
)

func newTransport(addr string) (*raft.NetworkTransport, error) {
	resolved, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("resolve raft addr %s: %w", addr, err)
	}
	transport, err := raft.NewTCPTransport(addr, resolved, transportMaxPool, transportTimeout, nil)
	if err != nil {
		return nil, fmt.Errorf("create tcp transport on %s: %w", addr, err)
	}
	return transport, nil
}
