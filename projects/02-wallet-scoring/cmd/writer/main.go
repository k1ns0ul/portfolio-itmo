package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/IBM/sarama"

	"github.com/andrey/wallet-scoring/internal/clickhouse"
	"github.com/andrey/wallet-scoring/internal/common"
	"github.com/andrey/wallet-scoring/internal/config"
	"github.com/andrey/wallet-scoring/internal/kafka"
	"github.com/andrey/wallet-scoring/internal/models"
	"github.com/andrey/wallet-scoring/migrations"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load("writer")
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	ctx, cancel := common.ShutdownContext()
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
	txRepo := clickhouse.NewTxRepo(ch)

	dlq, err := kafka.NewProducer(cfg.Kafka.Brokers)
	if err != nil {
		slog.Error("dlq producer", "err", err)
		os.Exit(1)
	}
	defer func() {
		closeCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		_ = dlq.Close(closeCtx)
	}()

	consumer, err := kafka.NewConsumer(kafka.ConsumerOptions{
		Brokers: cfg.Kafka.Brokers,
		GroupID: cfg.Kafka.GroupWriter,
		Topic:   cfg.Kafka.TopicRawTransactions,
		DLQ:     dlq,
		DLQName: cfg.Kafka.TopicDLQ,
	})
	if err != nil {
		slog.Error("consumer", "err", err)
		os.Exit(1)
	}
	defer consumer.Close()

	b := newBatcher(txRepo, cfg.Writer.BatchSize, cfg.Writer.BatchInterval)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		b.run(ctx)
	}()

	slog.Info("writer started", "topic", cfg.Kafka.TopicRawTransactions, "group", cfg.Kafka.GroupWriter)

	err = consumer.Subscribe(ctx, func(_ context.Context, msg *sarama.ConsumerMessage) error {
		env, err := kafka.DecodeEnvelope(msg.Value)
		if err != nil {
			return fmt.Errorf("decode: %w", err)
		}
		if env.Type != models.EventRawTransaction {
			return nil
		}
		var tx models.Transaction
		if err := env.Decode(&tx); err != nil {
			return fmt.Errorf("decode tx: %w", err)
		}
		if tx.BlockTime.IsZero() {
			tx.BlockTime = time.Now().UTC()
		}
		b.add(tx)
		return nil
	})
	if err != nil {
		slog.Error("subscribe", "err", err)
	}

	b.flush(context.Background())
	wg.Wait()

	consumed, retried, dropped := consumer.Metrics()
	slog.Info("writer stopped", "consumed", consumed, "retried", retried, "dropped", dropped)
}

type batcher struct {
	repo      *clickhouse.TxRepo
	limit     int
	interval  time.Duration
	mu        sync.Mutex
	buf       []models.Transaction
	ticker    *time.Ticker
}

func newBatcher(repo *clickhouse.TxRepo, limit int, interval time.Duration) *batcher {
	if limit <= 0 {
		limit = 1000
	}
	if interval <= 0 {
		interval = 5 * time.Second
	}
	return &batcher{
		repo:     repo,
		limit:    limit,
		interval: interval,
		buf:      make([]models.Transaction, 0, limit),
		ticker:   time.NewTicker(interval),
	}
}

func (b *batcher) add(tx models.Transaction) {
	b.mu.Lock()
	b.buf = append(b.buf, tx)
	full := len(b.buf) >= b.limit
	var pending []models.Transaction
	if full {
		pending = b.buf
		b.buf = make([]models.Transaction, 0, b.limit)
	}
	b.mu.Unlock()
	if full {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		b.write(ctx, pending)
	}
}

func (b *batcher) run(ctx context.Context) {
	defer b.ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-b.ticker.C:
			b.flush(ctx)
		}
	}
}

func (b *batcher) flush(ctx context.Context) {
	b.mu.Lock()
	if len(b.buf) == 0 {
		b.mu.Unlock()
		return
	}
	pending := b.buf
	b.buf = make([]models.Transaction, 0, b.limit)
	b.mu.Unlock()
	b.write(ctx, pending)
}

func (b *batcher) write(ctx context.Context, pending []models.Transaction) {
	if err := b.repo.InsertBatch(ctx, pending); err != nil {
		slog.Error("insert batch", "err", err, "size", len(pending))
		return
	}
	slog.Info("flushed", "size", len(pending))
}
