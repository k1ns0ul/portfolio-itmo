package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/andrey/cfa-bonds/internal/common"
	"github.com/andrey/cfa-bonds/internal/config"
	"github.com/andrey/cfa-bonds/internal/db"
	"github.com/andrey/cfa-bonds/internal/kafka"
	"github.com/andrey/cfa-bonds/internal/maturity"
	"github.com/andrey/cfa-bonds/internal/redis"
	"github.com/andrey/cfa-bonds/internal/repo"
)

func main() {
	once := flag.Bool("once", false, "run a single pass and exit")
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(log, *once); err != nil {
		log.Error("maturity worker terminated", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger, once bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.Workers.RunOnce {
		once = true
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
		log.Warn("redis unavailable", "err", err)
	}
	producer, err := kafka.NewProducer(cfg.Kafka.Brokers, cfg.Kafka.ClientID+"-maturity", log)
	if err != nil {
		log.Warn("kafka unavailable", "err", err)
	}
	if producer != nil {
		defer producer.Close()
	}

	svc := maturity.NewService(maturity.Deps{
		Pool:      pool,
		Issues:    repo.NewIssueRepo(pool),
		Investors: repo.NewInvestorRepo(pool),
		Positions: repo.NewPositionRepo(pool),
		Events:    repo.NewEventRepo(pool),
		Cache:     cache,
		Producer:  producer,
		Log:       log,
	})

	if once {
		return svc.ProcessMaturities(ctx, time.Now())
	}

	ticker := time.NewTicker(cfg.Workers.TickInterval)
	defer ticker.Stop()
	log.Info("maturity worker started in daemon mode", "interval", cfg.Workers.TickInterval.String())
	if err := svc.ProcessMaturities(ctx, time.Now()); err != nil {
		log.Error("initial maturity pass failed", "err", err)
	}
	for {
		select {
		case <-ctx.Done():
			log.Info("maturity worker stopping")
			return nil
		case <-ticker.C:
			if err := svc.ProcessMaturities(ctx, time.Now()); err != nil {
				log.Error("maturity pass failed", "err", err)
			}
		}
	}
}
