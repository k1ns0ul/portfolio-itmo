package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/IBM/sarama"

	"github.com/andrey/anomaly-detection/internal/config"
	"github.com/andrey/anomaly-detection/internal/kafka"
	"github.com/andrey/anomaly-detection/internal/models"
	rds "github.com/andrey/anomaly-detection/internal/redis"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load("extractor")
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	rdb, err := rds.NewClient(ctx, cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		slog.Error("redis", "err", err)
		os.Exit(1)
	}
	defer rdb.Close()

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
		GroupID:  cfg.Kafka.GroupExtractor,
		Topics:   []string{cfg.Kafka.TopicTx},
		DLQ:      producer,
		DLQTopic: cfg.Kafka.TopicDLQ,
	})
	if err != nil {
		slog.Error("kafka consumer", "err", err)
		os.Exit(1)
	}
	defer consumer.Close()

	store := rds.NewFeatureStore(rdb, cfg.Extractor.WindowShort, cfg.Extractor.WindowLong)

	slog.Info("extractor started",
		"in", cfg.Kafka.TopicTx,
		"out", cfg.Kafka.TopicFeatures,
		"group", cfg.Kafka.GroupExtractor,
	)

	err = consumer.Subscribe(ctx, func(hctx context.Context, msg *sarama.ConsumerMessage) error {
		var tx models.Transaction
		if err := json.Unmarshal(msg.Value, &tx); err != nil {
			return fmt.Errorf("decode tx: %w", err)
		}
		if tx.Timestamp.IsZero() {
			tx.Timestamp = time.Now().UTC()
		}
		features, err := store.ComputeFeatures(hctx, tx)
		if err != nil {
			return fmt.Errorf("features: %w", err)
		}
		payload, err := json.Marshal(features)
		if err != nil {
			return fmt.Errorf("encode features: %w", err)
		}
		producer.Send(cfg.Kafka.TopicFeatures, []byte(tx.ClientID), payload)
		return nil
	})
	if err != nil {
		slog.Error("subscribe", "err", err)
	}

	consumed, retried, dropped := consumer.Metrics()
	sent, failed := producer.Metrics()
	slog.Info("extractor stopped",
		"consumed", consumed, "retried", retried, "dropped", dropped,
		"sent", sent, "failed", failed,
	)
}
