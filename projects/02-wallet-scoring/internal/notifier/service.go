package notifier

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/redis/go-redis/v9"

	"github.com/andrey/wallet-scoring/internal/config"
	"github.com/andrey/wallet-scoring/internal/kafka"
	"github.com/andrey/wallet-scoring/internal/models"
)

type Service struct {
	cfg      config.NotifierConfig
	consumer *kafka.Consumer
	rdb      *redis.Client
	rules    []Rule

	mu       sync.Mutex
	previous map[string]models.WalletScore
}

func New(cfg config.NotifierConfig, consumer *kafka.Consumer, rdb *redis.Client) *Service {
	return &Service{
		cfg:      cfg,
		consumer: consumer,
		rdb:      rdb,
		rules:    DefaultRules(Config{ScoreThreshold: cfg.ScoreThreshold}),
		previous: make(map[string]models.WalletScore, 1024),
	}
}

func (s *Service) Run(ctx context.Context) error {
	slog.Info("notifier running", "threshold", s.cfg.ScoreThreshold, "channel", s.cfg.AlertChannel)
	return s.consumer.Subscribe(ctx, s.handle)
}

func (s *Service) handle(ctx context.Context, msg *sarama.ConsumerMessage) error {
	env, err := kafka.DecodeEnvelope(msg.Value)
	if err != nil {
		return fmt.Errorf("decode envelope: %w", err)
	}
	if env.Type != models.EventScoreUpdated {
		return nil
	}
	var next models.WalletScore
	if err := env.Decode(&next); err != nil {
		return fmt.Errorf("decode score: %w", err)
	}
	if !s.isWatched(ctx, next.Wallet) {
		return nil
	}

	prev := s.swap(next)
	alerts := s.evaluate(prev, next)
	for _, a := range alerts {
		s.publish(ctx, a)
	}
	return nil
}

func (s *Service) swap(next models.WalletScore) models.WalletScore {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.previous[next.Wallet]
	s.previous[next.Wallet] = next
	return prev
}

func (s *Service) evaluate(prev, next models.WalletScore) []models.Alert {
	var all []models.Alert
	for _, r := range s.rules {
		all = append(all, r(prev, next)...)
	}
	return all
}

func (s *Service) publish(ctx context.Context, a models.Alert) {
	payload, err := json.Marshal(a)
	if err != nil {
		slog.Error("alert marshal", "err", err)
		return
	}
	if err := s.rdb.Publish(ctx, s.cfg.AlertChannel, payload).Err(); err != nil {
		slog.Error("publish alert", "err", err)
		return
	}
	slog.Info("alert", "level", a.Level, "wallet", a.Wallet, "rule", a.Rule)
}

func (s *Service) isWatched(ctx context.Context, addr string) bool {
	if s.cfg.WatchListKey == "" {
		return true
	}
	pctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	n, err := s.rdb.Exists(pctx, s.cfg.WatchListKey).Result()
	if err != nil {
		return true
	}
	if n == 0 {
		return true
	}
	in, err := s.rdb.SIsMember(pctx, s.cfg.WatchListKey, addr).Result()
	if err != nil {
		return true
	}
	return in
}
