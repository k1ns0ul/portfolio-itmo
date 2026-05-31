package common

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func WaitForShutdown(ctx context.Context) os.Signal {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case s := <-sigCh:
		return s
	case <-ctx.Done():
		return nil
	}
}
