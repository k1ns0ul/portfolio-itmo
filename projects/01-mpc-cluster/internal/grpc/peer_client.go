package grpc

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type PeerClient struct {
	conn *grpc.ClientConn
	addr string
}

func DialPeer(addr string) (*PeerClient, error) {
	conn, err := grpc.Dial(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.CallContentSubtype(codecName)),
	)
	if err != nil {
		return nil, fmt.Errorf("dial peer %s: %w", addr, err)
	}
	return &PeerClient{conn: conn, addr: addr}, nil
}

func (p *PeerClient) Close() error {
	return p.conn.Close()
}

func (p *PeerClient) Exchange(ctx context.Context, in *Msg) (*Ack, error) {
	out := new(Ack)
	if err := p.conn.Invoke(ctx, "/"+PeerServiceName+"/Exchange", in, out); err != nil {
		return nil, fmt.Errorf("exchange with %s round %d: %w", p.addr, in.RoundNum, err)
	}
	return out, nil
}
