package grpc

import (
	"context"
	"fmt"
	"net"

	"google.golang.org/grpc"
)

type LocalExecutor interface {
	Execute(ctx context.Context, method, path string, body []byte) (int, []byte)
}

type Server struct {
	grpc *grpc.Server
	exec LocalExecutor
}

func NewServer(exec LocalExecutor) *Server {
	s := &Server{grpc: grpc.NewServer(), exec: exec}
	RegisterForwardServer(s.grpc, s)
	return s
}

func (s *Server) Forward(ctx context.Context, req *ForwardRequest) (*ForwardResponse, error) {
	status, body := s.exec.Execute(ctx, req.Method, req.Path, req.Body)
	return &ForwardResponse{StatusCode: int32(status), Body: body}, nil
}

func (s *Server) Serve(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen grpc on %s: %w", addr, err)
	}
	return s.grpc.Serve(lis)
}

func (s *Server) Stop() {
	s.grpc.GracefulStop()
}
