package clickhouse

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type Client struct {
	conn driver.Conn
}

func NewClient(ctx context.Context, dsn string) (*Client, error) {
	opts, err := clickhouse.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	opts.MaxOpenConns = 20
	opts.MaxIdleConns = 5
	opts.ConnMaxLifetime = time.Hour
	opts.DialTimeout = 5 * time.Second

	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := conn.Ping(pctx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Client{conn: conn}, nil
}

func (c *Client) Conn() driver.Conn { return c.conn }

func (c *Client) Ping(ctx context.Context) error { return c.conn.Ping(ctx) }

func (c *Client) Close() error { return c.conn.Close() }
