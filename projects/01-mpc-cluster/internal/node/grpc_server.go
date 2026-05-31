package node

import (
	"context"
	"log/slog"
	"sync"
	"time"

	mgrpc "github.com/andrey/mpc-cluster/internal/grpc"
	"google.golang.org/grpc"
)

type Buffer struct {
	mu   sync.Mutex
	cond *sync.Cond
	data map[int]map[int][]byte
}

func NewBuffer() *Buffer {
	b := &Buffer{data: make(map[int]map[int][]byte)}
	b.cond = sync.NewCond(&b.mu)
	return b
}

func (b *Buffer) Put(round, sender int, data []byte) {
	b.mu.Lock()
	if b.data[round] == nil {
		b.data[round] = make(map[int][]byte)
	}
	b.data[round][sender] = data
	b.cond.Broadcast()
	b.mu.Unlock()
}

func (b *Buffer) Wait(ctx context.Context, round, expected int, timeout time.Duration) map[int][]byte {
	deadline := time.Now().Add(timeout)
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	go func() {
		select {
		case <-timer.C:
		case <-ctx.Done():
		}
		b.mu.Lock()
		b.cond.Broadcast()
		b.mu.Unlock()
	}()

	b.mu.Lock()
	defer b.mu.Unlock()
	for {
		cur := b.data[round]
		if len(cur) >= expected || time.Now().After(deadline) || ctx.Err() != nil {
			return copyRound(cur)
		}
		b.cond.Wait()
	}
}

func copyRound(in map[int][]byte) map[int][]byte {
	out := make(map[int][]byte, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

type PeerService struct {
	buf *Buffer
	log *slog.Logger
}

func NewPeerService(buf *Buffer, log *slog.Logger) *PeerService {
	return &PeerService{buf: buf, log: log}
}

func (p *PeerService) Register(srv *grpc.Server) {
	mgrpc.RegisterPeerServer(srv, p)
}

func (p *PeerService) Exchange(ctx context.Context, in *mgrpc.Msg) (*mgrpc.Ack, error) {
	p.buf.Put(int(in.RoundNum), int(in.SenderID), in.Data)
	return &mgrpc.Ack{Ok: true}, nil
}
