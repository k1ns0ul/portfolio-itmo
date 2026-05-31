package grpc

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type CoordinatorClient struct {
	conn *grpc.ClientConn
}

func DialCoordinator(addr string) (*CoordinatorClient, error) {
	conn, err := grpc.Dial(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.CallContentSubtype(codecName)),
	)
	if err != nil {
		return nil, fmt.Errorf("dial coordinator %s: %w", addr, err)
	}
	return &CoordinatorClient{conn: conn}, nil
}

func (c *CoordinatorClient) Close() error {
	return c.conn.Close()
}

func (c *CoordinatorClient) RegisterNode(ctx context.Context, in *NodeInfo) (*Ack, error) {
	out := new(Ack)
	if err := c.conn.Invoke(ctx, "/"+CoordinatorServiceName+"/RegisterNode", in, out); err != nil {
		return nil, fmt.Errorf("register node %d: %w", in.NodeID, err)
	}
	return out, nil
}

func (c *CoordinatorClient) GetTask(ctx context.Context, in *TaskRequest) (*Task, error) {
	out := new(Task)
	if err := c.conn.Invoke(ctx, "/"+CoordinatorServiceName+"/GetTask", in, out); err != nil {
		return nil, fmt.Errorf("get task for session %s: %w", in.SessionID, err)
	}
	return out, nil
}

func (c *CoordinatorClient) SubmitResult(ctx context.Context, in *ResultMsg) (*Ack, error) {
	out := new(Ack)
	if err := c.conn.Invoke(ctx, "/"+CoordinatorServiceName+"/SubmitResult", in, out); err != nil {
		return nil, fmt.Errorf("submit result: %w", err)
	}
	return out, nil
}

var heartbeatStreamDesc = &grpc.StreamDesc{
	StreamName:    "Heartbeat",
	ServerStreams: true,
	ClientStreams: true,
}

func (c *CoordinatorClient) Heartbeat(ctx context.Context) (grpc.ClientStream, error) {
	stream, err := c.conn.NewStream(ctx, heartbeatStreamDesc, "/"+CoordinatorServiceName+"/Heartbeat")
	if err != nil {
		return nil, fmt.Errorf("open heartbeat stream: %w", err)
	}
	return stream, nil
}
