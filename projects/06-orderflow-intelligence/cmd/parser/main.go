package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/andrey/orderflow-intelligence/internal/config"
	"github.com/andrey/orderflow-intelligence/internal/kafka"
	pb "github.com/andrey/orderflow-intelligence/internal/proto"
	"github.com/andrey/orderflow-intelligence/internal/solana"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load("parser")
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

	geyser, err := solana.NewGeyser(ctx, cfg.Geyser.Endpoint, cfg.Geyser.Token)
	if err != nil {
		slog.Error("geyser", "err", err)
		os.Exit(1)
	}
	defer geyser.Close()

	includes := []string{solana.RaydiumV4, solana.RaydiumCPMM, solana.OrcaWhirlpool, solana.JupiterV6}
	updates := geyser.Subscribe(ctx, solana.Filter{
		AccountIncludes: includes,
		Commitment:      pb.CommitmentFromString(cfg.Geyser.Commitment),
	})

	slog.Info("parser started", "endpoint", cfg.Geyser.Endpoint, "topic", cfg.Kafka.TopicSwaps)

	statsTicker := time.NewTicker(30 * time.Second)
	defer statsTicker.Stop()

	var emitted uint64
	for {
		select {
		case <-ctx.Done():
			received, reconnects, errs := geyser.Metrics()
			sent, failed := producer.Metrics()
			slog.Info("parser stopping",
				"received", received, "reconnects", reconnects, "errs", errs,
				"sent", sent, "failed", failed, "emitted", emitted,
			)
			return
		case <-statsTicker.C:
			received, reconnects, errs := geyser.Metrics()
			sent, failed := producer.Metrics()
			slog.Info("parser progress",
				"received", received, "reconnects", reconnects, "errs", errs,
				"sent", sent, "failed", failed, "emitted", emitted,
			)
		case bundle, ok := <-updates:
			if !ok {
				return
			}
			for _, swap := range bundle.Swaps {
				payload, err := json.Marshal(swap)
				if err != nil {
					continue
				}
				producer.Send(cfg.Kafka.TopicSwaps, []byte(swap.Pair), payload)
				emitted++
			}
		}
	}
}
