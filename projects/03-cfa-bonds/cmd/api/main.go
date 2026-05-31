package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/andrey/cfa-bonds/internal/api"
	"github.com/andrey/cfa-bonds/internal/auth"
	"github.com/andrey/cfa-bonds/internal/common"
	"github.com/andrey/cfa-bonds/internal/config"
	"github.com/andrey/cfa-bonds/internal/db"
	"github.com/andrey/cfa-bonds/internal/kafka"
	"github.com/andrey/cfa-bonds/internal/redis"
	"github.com/andrey/cfa-bonds/internal/repo"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(log); err != nil {
		log.Error("api terminated", "err", err)
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
	if err := db.Migrate(ctx, pool); err != nil {
		return err
	}
	log.Info("database ready")

	cache, err := redis.New(ctx, cfg.Redis)
	if err != nil {
		return err
	}
	defer cache.Close()

	producer, err := kafka.NewProducer(cfg.Kafka.Brokers, cfg.Kafka.ClientID, log)
	if err != nil {
		log.Warn("kafka producer unavailable, trades will not be queued", "err", err)
	}
	if producer != nil {
		defer producer.Close()
	}

	srv := api.NewServer(api.ServerDeps{
		Cfg:       cfg.API,
		Pool:      pool,
		Issuers:   repo.NewIssuerRepo(pool),
		Investors: repo.NewInvestorRepo(pool),
		Issues:    repo.NewIssueRepo(pool),
		Positions: repo.NewPositionRepo(pool),
		Trades:    repo.NewTradeRepo(pool),
		Coupons:   repo.NewCouponRepo(pool),
		Events:    repo.NewEventRepo(pool),
		Cache:     cache,
		Limiter:   redis.NewRateLimiter(cache, cfg.API.RateLimitPerMin),
		Producer:  producer,
		Auth:      auth.NewIssuer(cfg.API.JWTSecret, 24*time.Hour),
		Log:       log,
	})

	httpSrv := &http.Server{
		Addr:              cfg.API.ListenAddr,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Info("api listening", "addr", cfg.API.ListenAddr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server failed", "err", err)
			cancel()
		}
	}()

	<-ctx.Done()
	log.Info("shutting down api")

	shutCtx, shutCancel := context.WithTimeout(context.Background(), cfg.API.ShutdownGrace)
	defer shutCancel()
	if err := httpSrv.Shutdown(shutCtx); err != nil {
		return err
	}
	log.Info("api stopped")
	return nil
}
