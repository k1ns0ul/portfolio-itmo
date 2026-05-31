package clickhouse

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"github.com/andrey/wallet-scoring/internal/common"
)

type ClientMetrics struct {
	Queries      atomic.Uint64
	Errors       atomic.Uint64
	LastLatencyN atomic.Int64
}

type Client struct {
	conn    driver.Conn
	metrics ClientMetrics
}

func NewClient(ctx context.Context, dsn string) (*Client, error) {
	opts, err := clickhouse.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("clickhouse parse dsn: %w", err)
	}
	opts.MaxOpenConns = 20
	opts.MaxIdleConns = 5
	opts.ConnMaxLifetime = time.Hour
	opts.DialTimeout = 5 * time.Second
	opts.Compression = &clickhouse.Compression{Method: clickhouse.CompressionLZ4}

	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("clickhouse open: %w", err)
	}

	err = common.Retry(ctx, 5, time.Second, func(ctx context.Context) error {
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		return conn.Ping(pingCtx)
	})
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("clickhouse ping: %w", err)
	}

	return &Client{conn: conn}, nil
}

func (c *Client) Conn() driver.Conn { return c.conn }

func (c *Client) Ping(ctx context.Context) error { return c.conn.Ping(ctx) }

func (c *Client) Close() error { return c.conn.Close() }

func (c *Client) Metrics() (queries, errs uint64, lastLatencyMs int64) {
	return c.metrics.Queries.Load(), c.metrics.Errors.Load(), c.metrics.LastLatencyN.Load() / int64(time.Millisecond)
}

func (c *Client) recordLatency(start time.Time, err error) {
	c.metrics.Queries.Add(1)
	c.metrics.LastLatencyN.Store(int64(time.Since(start)))
	if err != nil {
		c.metrics.Errors.Add(1)
	}
}
