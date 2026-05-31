package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/andrey/wallet-scoring/internal/common"
	"github.com/andrey/wallet-scoring/internal/config"
	"github.com/andrey/wallet-scoring/internal/kafka"
	"github.com/andrey/wallet-scoring/internal/models"
	pb "github.com/andrey/wallet-scoring/internal/proto"
	"github.com/andrey/wallet-scoring/internal/solana"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load("ingester")
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	ctx, cancel := common.ShutdownContext()
	defer cancel()

	producer, err := kafka.NewProducer(cfg.Kafka.Brokers)
	if err != nil {
		slog.Error("kafka", "err", err)
		os.Exit(1)
	}
	defer func() {
		closeCtx, c := context.WithTimeout(context.Background(), 10*time.Second)
		defer c()
		if err := producer.Close(closeCtx); err != nil {
			slog.Error("producer close", "err", err)
		}
	}()

	geyser, err := solana.NewGeyser(ctx, cfg.Ingester.GRPCEndpoint, cfg.Ingester.GRPCToken)
	if err != nil {
		slog.Error("geyser", "err", err)
		os.Exit(1)
	}
	defer geyser.Close()

	updates := geyser.Subscribe(ctx, solana.Filter{
		AccountIncludes: solana.TrackedPrograms(),
		Commitment:      pb.CommitmentFromString(cfg.Ingester.Commitment),
	})

	slog.Info("ingester started", "endpoint", cfg.Ingester.GRPCEndpoint, "topic", cfg.Kafka.TopicRawTransactions)

	statsTicker := time.NewTicker(30 * time.Second)
	defer statsTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			rec, rcn, errs := geyser.Metrics()
			sent, failed := producer.Metrics()
			slog.Info("ingester stopping", "received", rec, "reconnects", rcn, "errors", errs, "sent", sent, "failed", failed)
			return
		case <-statsTicker.C:
			rec, rcn, errs := geyser.Metrics()
			sent, failed := producer.Metrics()
			slog.Info("ingester progress", "received", rec, "reconnects", rcn, "errors", errs, "sent", sent, "failed", failed)
		case tx, ok := <-updates:
			if !ok {
				return
			}
			env, err := models.NewEnvelope(models.EventRawTransaction, "ingester", tx)
			if err != nil {
				slog.Debug("envelope", "err", err)
				continue
			}
			payload, err := kafka.EncodeEnvelope(env)
			if err != nil {
				continue
			}
			producer.Send(cfg.Kafka.TopicRawTransactions, []byte(tx.Signature), payload)
		}
	}
}
