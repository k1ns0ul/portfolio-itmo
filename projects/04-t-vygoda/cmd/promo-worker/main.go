package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/andrey/t-vygoda/internal/config"
	"github.com/andrey/t-vygoda/internal/db"
	"github.com/andrey/t-vygoda/internal/kafka"
	"github.com/andrey/t-vygoda/internal/repo"
	"github.com/andrey/t-vygoda/internal/workers"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load("promo-worker")
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pg, err := db.Connect(ctx, cfg.DB.DSN)
	if err != nil {
		slog.Error("postgres", "err", err)
		os.Exit(1)
	}
	defer pg.Close()

	producer, err := kafka.NewProducer(cfg.Kafka.Brokers)
	if err != nil {
		slog.Error("kafka producer", "err", err)
		os.Exit(1)
	}
	defer func() {
		closeCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		_ = producer.Close(closeCtx)
	}()

	consumer, err := kafka.NewConsumer(kafka.Options{
		Brokers:  cfg.Kafka.Brokers,
		GroupID:  cfg.Kafka.GroupPromoWorker,
		Topics:   []string{cfg.Kafka.TopicPromos, cfg.Kafka.TopicPurchases},
		DLQ:      producer,
		DLQTopic: cfg.Kafka.TopicDLQ,
	})
	if err != nil {
		slog.Error("kafka consumer", "err", err)
		os.Exit(1)
	}
	defer consumer.Close()

	handler := workers.NewPromoHandler(
		cfg.Kafka,
		repo.NewPromoRepo(pg),
		repo.NewPurchaseRepo(pg),
		repo.NewPartnerRepo(pg),
		producer,
	)

	slog.Info("promo-worker started", "topics", []string{cfg.Kafka.TopicPromos, cfg.Kafka.TopicPurchases})
	if err := consumer.Subscribe(ctx, handler.Handle); err != nil {
		slog.Error("subscribe", "err", err)
	}
	consumed, retried, dropped := consumer.Metrics()
	slog.Info("promo-worker stopped", "consumed", consumed, "retried", retried, "dropped", dropped)
}
