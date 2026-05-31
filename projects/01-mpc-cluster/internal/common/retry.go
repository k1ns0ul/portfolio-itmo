package common

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"
)

type RetryConfig struct {
	Attempts int
	Base     time.Duration
	Max      time.Duration
}

func DefaultRetry() RetryConfig {
	return RetryConfig{Attempts: 5, Base: 200 * time.Millisecond, Max: 5 * time.Second}
}

func Retry(ctx context.Context, cfg RetryConfig, fn func() error) error {
	var last error
	delay := cfg.Base
	for attempt := 1; attempt <= cfg.Attempts; attempt++ {
		last = fn()
		if last == nil {
			return nil
		}
		if attempt == cfg.Attempts {
			break
		}
		sleep := delay + jitter(delay)
		if sleep > cfg.Max {
			sleep = cfg.Max
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry aborted after %d attempts: %w", attempt, ctx.Err())
		case <-time.After(sleep):
		}
		delay *= 2
		if delay > cfg.Max {
			delay = cfg.Max
		}
	}
	return fmt.Errorf("exhausted %d attempts: %w", cfg.Attempts, last)
}

func jitter(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(d)))
	if err != nil {
		return 0
	}
	return time.Duration(n.Int64())
}
