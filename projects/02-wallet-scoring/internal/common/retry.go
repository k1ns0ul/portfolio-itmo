package common

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

func Retry(ctx context.Context, maxAttempts int, baseDelay time.Duration, fn func(ctx context.Context) error) error {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	delay := baseDelay
	var last error
	for i := 0; i < maxAttempts; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := fn(ctx)
		if err == nil {
			return nil
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		last = err
		if i == maxAttempts-1 {
			break
		}
		slog.Debug("retry", "attempt", i+1, "delay", delay.String(), "err", err)
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
		delay *= 2
		if delay > 30*time.Second {
			delay = 30 * time.Second
		}
	}
	return fmt.Errorf("after %d attempts: %w", maxAttempts, last)
}
