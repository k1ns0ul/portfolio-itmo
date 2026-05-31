package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/andrey/orderflow-intelligence/internal/config"
	"github.com/andrey/orderflow-intelligence/internal/kafka"
	"github.com/andrey/orderflow-intelligence/internal/mockgen"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load("mockgen")
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	producer, err := kafka.NewProducer(cfg.Kafka.Brokers)
	if err != nil {
		slog.Error("kafka", "err", err)
		os.Exit(1)
	}
	defer func() {
		closeCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		_ = producer.Close(closeCtx)
	}()

	gen := mockgen.New(cfg.MockGen, producer, cfg.Kafka.TopicSwaps)
	if err := gen.Run(ctx); err != nil {
		slog.Error("mockgen", "err", err)
		os.Exit(1)
	}
}
