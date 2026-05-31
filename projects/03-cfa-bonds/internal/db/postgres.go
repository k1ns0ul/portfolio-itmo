package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/andrey/cfa-bonds/internal/common"
	"github.com/andrey/cfa-bonds/internal/config"
)

func Connect(ctx context.Context, cfg config.DBConfig) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse db dsn: %w", err)
	}
	if cfg.MaxConns > 0 {
		poolCfg.MaxConns = cfg.MaxConns
	}
	if cfg.MinConns > 0 {
		poolCfg.MinConns = cfg.MinConns
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("init pgx pool: %w", err)
	}

	rc := common.RetryConfig{Attempts: cfg.PingRetries, Base: cfg.PingInterval, Max: cfg.PingInterval * 4}
	if err := common.Retry(ctx, rc, func() error { return pool.Ping(ctx) }); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres not reachable: %w", err)
	}
	return pool, nil
}
