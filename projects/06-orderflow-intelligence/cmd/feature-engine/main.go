package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/IBM/sarama"

	"github.com/andrey/orderflow-intelligence/internal/clickhouse"
	"github.com/andrey/orderflow-intelligence/internal/config"
	"github.com/andrey/orderflow-intelligence/internal/features"
	"github.com/andrey/orderflow-intelligence/internal/kafka"
	"github.com/andrey/orderflow-intelligence/internal/models"
	rds "github.com/andrey/orderflow-intelligence/internal/redis"
	"github.com/andrey/orderflow-intelligence/migrations"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load("engine")
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	ch, err := clickhouse.NewClient(ctx, cfg.ClickHouse.DSN)
	if err != nil {
		slog.Error("clickhouse", "err", err)
		os.Exit(1)
	}
	defer ch.Close()

	if cfg.ClickHouse.Migrate {
		if err := clickhouse.Migrate(ctx, ch, migrations.FS, migrations.Dir); err != nil {
			slog.Error("migrate", "err", err)
			os.Exit(1)
		}
	}

	rdb, err := rds.NewClient(ctx, cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		slog.Error("redis", "err", err)
		os.Exit(1)
	}
	defer rdb.Close()

	dlq, err := kafka.NewProducer(cfg.Kafka.Brokers)
	if err != nil {
		slog.Error("kafka dlq", "err", err)
		os.Exit(1)
	}
	defer func() {
		closeCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		_ = dlq.Close(closeCtx)
	}()

	consumer, err := kafka.NewConsumer(kafka.Options{
		Brokers:  cfg.Kafka.Brokers,
		GroupID:  cfg.Kafka.GroupEngine,
		Topics:   []string{cfg.Kafka.TopicSwaps},
		DLQ:      dlq,
		DLQTopic: cfg.Kafka.TopicDLQ,
	})
	if err != nil {
		slog.Error("kafka consumer", "err", err)
		os.Exit(1)
	}
	defer consumer.Close()

	repo := clickhouse.NewRepo(ch)
	cache := rds.NewCache(rdb, 5*time.Minute)
	engine := features.NewEngine(cfg.Engine.Intervals, repo, cache, cfg.Engine.BatchSize)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		engine.Run(ctx)
	}()

	slog.Info("engine started",
		"topic", cfg.Kafka.TopicSwaps,
		"intervals", cfg.Engine.Intervals,
	)
	err = consumer.Subscribe(ctx, func(_ context.Context, msg *sarama.ConsumerMessage) error {
		var swap models.SwapEvent
		if err := json.Unmarshal(msg.Value, &swap); err != nil {
			return fmt.Errorf("decode swap: %w", err)
		}
		if swap.BlockTime.IsZero() {
			swap.BlockTime = time.Now().UTC()
		}
		engine.Push(swap)
		return nil
	})
	if err != nil {
		slog.Error("subscribe", "err", err)
	}
	wg.Wait()
	consumed, retried, dropped := consumer.Metrics()
	slog.Info("engine stopped", "consumed", consumed, "retried", retried, "dropped", dropped)
}
