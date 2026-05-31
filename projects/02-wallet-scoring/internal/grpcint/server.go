package agg

import (
	"context"

	"google.golang.org/grpc"
)

type Handler interface {
	WalletStats(ctx context.Context, address string) (*WalletStatsResponse, error)
	TopWallets(ctx context.Context, limit, offset uint32) (*TopWalletsResponse, error)
	RefreshWallet(ctx context.Context, address string) error
}

type server struct {
	h Handler
}

func RegisterServer(s *grpc.Server, h Handler) {
	s.RegisterService(&serviceDesc, &server{h: h})
}

func (s *server) getWalletStats(ctx context.Context, dec func(any) error) (any, error) {
	in := new(GetWalletStatsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	return s.h.WalletStats(ctx, in.Address)
}

func (s *server) getTopWallets(ctx context.Context, dec func(any) error) (any, error) {
	in := new(GetTopWalletsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	return s.h.TopWallets(ctx, in.Limit, in.Offset)
}

func (s *server) refreshWallet(ctx context.Context, dec func(any) error) (any, error) {
	in := new(RefreshWalletRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if err := s.h.RefreshWallet(ctx, in.Address); err != nil {
		return nil, err
	}
	return &Empty{}, nil
}

var serviceDesc = grpc.ServiceDesc{
	ServiceName: "agg.AggregatorService",
	HandlerType: (*Handler)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetWalletStats",
			Handler: func(srv any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
				return srv.(*server).getWalletStats(ctx, dec)
			},
		},
		{
			MethodName: "GetTopWallets",
			Handler: func(srv any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
				return srv.(*server).getTopWallets(ctx, dec)
			},
		},
		{
			MethodName: "RefreshWallet",
			Handler: func(srv any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
				return srv.(*server).refreshWallet(ctx, dec)
			},
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "aggregator.proto",
}
