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

func SignalContext(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	go func() {
		WaitForShutdown(ctx)
		cancel()
	}()
	return ctx, cancel
}
