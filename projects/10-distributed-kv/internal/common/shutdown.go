package common

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func WaitForShutdown(ctx context.Context) os.Signal {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(ch)
	select {
	case s := <-ch:
		return s
	case <-ctx.Done():
		return nil
	}
}
