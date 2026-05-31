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
	return RetryConfig{Attempts: 6, Base: 250 * time.Millisecond, Max: 8 * time.Second}
}

func Retry(ctx context.Context, cfg RetryConfig, op func() error) error {
	var last error
	delay := cfg.Base
	for i := 1; i <= cfg.Attempts; i++ {
		if last = op(); last == nil {
			return nil
		}
		if i == cfg.Attempts {
			break
		}
		wait := delay + jitter(delay)
		if wait > cfg.Max {
			wait = cfg.Max
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled on attempt %d: %w", i, ctx.Err())
		case <-time.After(wait):
		}
		delay *= 2
		if delay > cfg.Max {
			delay = cfg.Max
		}
	}
	return fmt.Errorf("gave up after %d attempts: %w", cfg.Attempts, last)
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
