package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/andrey/cfa-bonds/internal/common"
	"github.com/andrey/cfa-bonds/internal/config"
	"github.com/andrey/cfa-bonds/internal/db"
	"github.com/andrey/cfa-bonds/internal/kafka"
	"github.com/andrey/cfa-bonds/internal/metrics"
	"github.com/andrey/cfa-bonds/internal/redis"
	"github.com/andrey/cfa-bonds/internal/repo"
	"github.com/andrey/cfa-bonds/internal/settlement"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(log); err != nil {
		log.Error("settlement worker terminated", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, cancel := common.SignalContext(context.Background())
	defer cancel()

	pool, err := db.Connect(ctx, cfg.DB)
	if err != nil {
		return err
	}
	defer pool.Close()

	cache, err := redis.New(ctx, cfg.Redis)
	if err != nil {
		log.Warn("redis unavailable, cache disabled", "err", err)
	}

	producer, err := kafka.NewProducer(cfg.Kafka.Brokers, cfg.Kafka.ClientID+"-settle", log)
	if err != nil {
		return fmt.Errorf("init producer: %w", err)
	}
	defer producer.Close()

	engine := settlement.NewEngine(settlement.Deps{
		Pool:      pool,
		Issues:    repo.NewIssueRepo(pool),
		Investors: repo.NewInvestorRepo(pool),
		Positions: repo.NewPositionRepo(pool),
		Trades:    repo.NewTradeRepo(pool),
		Coupons:   repo.NewCouponRepo(pool),
		Events:    repo.NewEventRepo(pool),
		Cache:     cache,
		Producer:  producer,
		Log:       log,
		Observe:   metrics.ObserveSettlement,
	})

	handler := func(ctx context.Context, key, value []byte) error {
		tradeID := extractTradeID(key, value)
		if tradeID == "" {
			return fmt.Errorf("cannot determine trade id from message")
		}
		return engine.ProcessTrade(ctx, tradeID)
	}

	consumer, err := kafka.NewConsumer(kafka.ConsumerOptions{
		Brokers:  cfg.Kafka.Brokers,
		GroupID:  cfg.Kafka.ConsumerGroup,
		ClientID: cfg.Kafka.ClientID + "-settle",
		Topics:   []string{kafka.TopicTradeSubmitted},
		DLQ:      producer,
		DLQTopic: cfg.Workers.SettlementDLQ,
		MaxRetry: 3,
	}, handler, log)
	if err != nil {
		return err
	}
	defer consumer.Close()

	log.Info("settlement worker started", "group", cfg.Kafka.ConsumerGroup)
	return consumer.Run(ctx)
}

func extractTradeID(key, value []byte) string {
	if len(key) > 0 {
		return string(key)
	}
	var probe struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(value, &probe); err == nil {
		return probe.ID
	}
	return ""
}
