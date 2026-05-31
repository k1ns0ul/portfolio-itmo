package node

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	mgrpc "github.com/andrey/mpc-cluster/internal/grpc"
)

type PeerNetwork struct {
	myID      int
	sessionID string
	timeout   time.Duration
	buf       *Buffer

	mu      sync.Mutex
	clients map[int]*mgrpc.PeerClient
	round   int64
}

func NewPeerNetwork(myID int, sessionID string, buf *Buffer, timeout time.Duration) *PeerNetwork {
	return &PeerNetwork{
		myID:      myID,
		sessionID: sessionID,
		timeout:   timeout,
		buf:       buf,
		clients:   make(map[int]*mgrpc.PeerClient),
	}
}

func (p *PeerNetwork) Connect(peers map[int]string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for id, addr := range peers {
		if id == p.myID {
			continue
		}
		if _, ok := p.clients[id]; ok {
			continue
		}
		cli, err := mgrpc.DialPeer(addr)
		if err != nil {
			return fmt.Errorf("connect peer %d (%s): %w", id, addr, err)
		}
		p.clients[id] = cli
	}
	return nil
}

func (p *PeerNetwork) NextRound() int {
	return int(atomic.AddInt64(&p.round, 1))
}

func (p *PeerNetwork) Broadcast(ctx context.Context, roundNum int, data []byte) error {
	p.mu.Lock()
	targets := make([]*mgrpc.PeerClient, 0, len(p.clients))
	for _, c := range p.clients {
		targets = append(targets, c)
	}
	p.mu.Unlock()

	msg := &mgrpc.Msg{
		RoundNum:  int32(roundNum),
		SenderID:  int32(p.myID),
		SessionID: p.sessionID,
		Data:      data,
	}
	for _, c := range targets {
		if _, err := c.Exchange(ctx, msg); err != nil {
			return fmt.Errorf("round %d broadcast: %w", roundNum, err)
		}
	}
	return nil
}

func (p *PeerNetwork) Gather(ctx context.Context, roundNum int) (map[int][]byte, error) {
	p.mu.Lock()
	expected := len(p.clients)
	p.mu.Unlock()

	got := p.buf.Wait(ctx, roundNum, expected, p.timeout)
	if len(got) < expected {
		return got, fmt.Errorf("round %d gathered %d of %d peers within %v", roundNum, len(got), expected, p.timeout)
	}
	return got, nil
}

func (p *PeerNetwork) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for id, c := range p.clients {
		c.Close()
		delete(p.clients, id)
	}
}
