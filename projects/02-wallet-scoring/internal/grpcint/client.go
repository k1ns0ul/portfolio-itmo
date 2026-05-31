package agg

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	"github.com/andrey/wallet-scoring/internal/common"
)

type Client struct {
	conn *grpc.ClientConn
}

func Dial(ctx context.Context, addr string) (*Client, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                30 * time.Second,
			Timeout:             10 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(common.Codec{})),
	}
	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(dialCtx, addr, opts...)
	if err != nil {
		return nil, fmt.Errorf("dial aggregator %s: %w", addr, err)
	}
	return &Client{conn: conn}, nil
}

func (c *Client) Close() error { return c.conn.Close() }

func (c *Client) WalletStats(ctx context.Context, addr string) (*WalletStatsResponse, error) {
	out := &WalletStatsResponse{}
	err := c.invoke(ctx, "GetWalletStats", &GetWalletStatsRequest{Address: addr}, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) TopWallets(ctx context.Context, limit, offset uint32) (*TopWalletsResponse, error) {
	out := &TopWalletsResponse{}
	err := c.invoke(ctx, "GetTopWallets", &GetTopWalletsRequest{Limit: limit, Offset: offset}, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) Refresh(ctx context.Context, addr string) error {
	return c.invoke(ctx, "RefreshWallet", &RefreshWalletRequest{Address: addr}, &Empty{})
}

func (c *Client) invoke(ctx context.Context, method string, in, out any) error {
	return common.Retry(ctx, 3, 200*time.Millisecond, func(ctx context.Context) error {
		return c.conn.Invoke(ctx, "/agg.AggregatorService/"+method, in, out)
	})
}
