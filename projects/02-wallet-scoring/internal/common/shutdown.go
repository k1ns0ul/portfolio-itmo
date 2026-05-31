package common

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func ShutdownContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}
