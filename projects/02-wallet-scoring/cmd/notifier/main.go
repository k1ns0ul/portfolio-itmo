package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/andrey/wallet-scoring/internal/common"
	"github.com/andrey/wallet-scoring/internal/config"
	"github.com/andrey/wallet-scoring/internal/kafka"
	"github.com/andrey/wallet-scoring/internal/notifier"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load("notifier")
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	ctx, cancel := common.ShutdownContext()
	defer cancel()

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer rdb.Close()

	consumer, err := kafka.NewConsumer(kafka.ConsumerOptions{
		Brokers: cfg.Kafka.Brokers,
		GroupID: cfg.Kafka.GroupNotifier,
		Topic:   cfg.Notifier.ScoreUpdateTopic,
	})
	if err != nil {
		slog.Error("kafka", "err", err)
		os.Exit(1)
	}
	defer consumer.Close()

	svc := notifier.New(cfg.Notifier, consumer, rdb)
	if err := svc.Run(ctx); err != nil {
		slog.Error("notifier", "err", err)
	}

	consumed, retried, dropped := consumer.Metrics()
	slog.Info("notifier stopped", "consumed", consumed, "retried", retried, "dropped", dropped, "uptime", time.Since(time.Now()).String())
}
