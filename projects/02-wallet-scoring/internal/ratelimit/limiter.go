package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Limiter struct {
	rdb    *redis.Client
	prefix string
}

func New(rdb *redis.Client, prefix string) *Limiter {
	if prefix == "" {
		prefix = "rl"
	}
	return &Limiter{rdb: rdb, prefix: prefix}
}

func (l *Limiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, error) {
	if limit <= 0 {
		return true, 0, nil
	}
	full := l.prefix + ":" + key
	pipe := l.rdb.TxPipeline()
	incr := pipe.Incr(ctx, full)
	pipe.Expire(ctx, full, window)
	if _, err := pipe.Exec(ctx); err != nil {
		return false, 0, fmt.Errorf("ratelimit exec: %w", err)
	}
	n, err := incr.Result()
	if err != nil {
		return false, 0, err
	}
	remaining := limit - int(n)
	if remaining < 0 {
		remaining = 0
	}
	return n <= int64(limit), remaining, nil
}

func (l *Limiter) Reset(ctx context.Context, key string) error {
	return l.rdb.Del(ctx, l.prefix+":"+key).Err()
}
