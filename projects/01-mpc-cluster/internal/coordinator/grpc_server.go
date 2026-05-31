package coordinator

import (
	"context"
	"errors"
	"io"
	"log/slog"

	mgrpc "github.com/andrey/mpc-cluster/internal/grpc"
	"github.com/andrey/mpc-cluster/internal/session"
	"google.golang.org/grpc"
)

type GRPCServer struct {
	manager *session.SessionManager
	log     *slog.Logger
}

func NewGRPCServer(m *session.SessionManager, log *slog.Logger) *GRPCServer {
	return &GRPCServer{manager: m, log: log}
}

func (s *GRPCServer) Register(srv *grpc.Server) {
	mgrpc.RegisterCoordinatorServer(srv, s)
}

func (s *GRPCServer) RegisterNode(ctx context.Context, in *mgrpc.NodeInfo) (*mgrpc.Ack, error) {
	if err := s.manager.RegisterNode(ctx, in.SessionID, int(in.NodeID), in.Addr); err != nil {
		s.log.Warn("register failed", "session", in.SessionID, "node", in.NodeID, "err", err)
		return &mgrpc.Ack{Ok: false, Message: err.Error()}, nil
	}
	return &mgrpc.Ack{Ok: true, Message: "registered"}, nil
}

func (s *GRPCServer) GetTask(ctx context.Context, in *mgrpc.TaskRequest) (*mgrpc.Task, error) {
	return s.manager.FetchTask(in.SessionID, int(in.NodeID)), nil
}

func (s *GRPCServer) SubmitResult(ctx context.Context, in *mgrpc.ResultMsg) (*mgrpc.Ack, error) {
	if err := s.manager.AcceptResult(in.SessionID, int(in.NodeID), in.Value, in.Err); err != nil {
		return &mgrpc.Ack{Ok: false, Message: err.Error()}, nil
	}
	return &mgrpc.Ack{Ok: true, Message: "accepted"}, nil
}

func (s *GRPCServer) Heartbeat(stream grpc.ServerStream) error {
	for {
		ping := new(mgrpc.HeartbeatPing)
		if err := stream.RecvMsg(ping); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if err := stream.SendMsg(&mgrpc.HeartbeatPong{Ok: true, Command: "continue"}); err != nil {
			return err
		}
	}
}
