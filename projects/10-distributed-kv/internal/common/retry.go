package common

import (
	"context"
	"fmt"
	"time"
)

func RetryUntil(ctx context.Context, attempts int, delay time.Duration, op func() error) error {
	var last error
	for i := 0; i < attempts; i++ {
		if last = op(); last == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled after %d attempts: %w", i+1, ctx.Err())
		case <-time.After(delay):
		}
	}
	return fmt.Errorf("operation failed after %d attempts: %w", attempts, last)
}

func WaitFor(ctx context.Context, timeout, poll time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(poll):
		}
	}
	return cond()
}
