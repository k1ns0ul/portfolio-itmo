package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/andrey/anomaly-detection/internal/config"
	"github.com/andrey/anomaly-detection/internal/kafka"
	"github.com/andrey/anomaly-detection/internal/simulator"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load("simulator")
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

	gen := simulator.New(cfg.Simulator, producer, cfg.Kafka.TopicTx)
	if err := gen.Run(ctx); err != nil {
		slog.Error("simulator", "err", err)
		os.Exit(1)
	}
}
