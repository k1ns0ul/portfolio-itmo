package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/andrey/t-vygoda/internal/api"
	"github.com/andrey/t-vygoda/internal/auth"
	"github.com/andrey/t-vygoda/internal/config"
	"github.com/andrey/t-vygoda/internal/db"
	"github.com/andrey/t-vygoda/internal/kafka"
	rds "github.com/andrey/t-vygoda/internal/redis"
	"github.com/andrey/t-vygoda/internal/recommender"
	"github.com/andrey/t-vygoda/internal/repo"
	"github.com/andrey/t-vygoda/migrations"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load("api")
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

	if cfg.DB.Migrate {
		if err := db.Migrate(ctx, pg, migrations.FS, migrations.Dir); err != nil {
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

	producer, err := kafka.NewProducer(cfg.Kafka.Brokers)
	if err != nil {
		slog.Error("kafka", "err", err)
		os.Exit(1)
	}
	defer func() {
		closeCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		_ = producer.Close(closeCtx)
	}()

	visitCh := make(chan int64, 4096)
	streaks := rds.NewStreaks(rdb)
	api.StartVisitWorker(ctx, streaks, visitCh, 2*time.Second)

	deps := &api.Deps{
		Cfg:             cfg,
		Tokenizer:       auth.NewTokenizer(cfg.JWT.Secret, cfg.JWT.Issuer, cfg.JWT.AccessTTL, cfg.JWT.RefreshTTL),
		Users:           repo.NewUserRepo(pg),
		Posts:           repo.NewPostRepo(pg),
		Promos:          repo.NewPromoRepo(pg),
		Partners:        repo.NewPartnerRepo(pg),
		Purchases:       repo.NewPurchaseRepo(pg),
		Referrals:       repo.NewReferralRepo(pg),
		CFA:             repo.NewCFARepo(pg),
		Categories:      repo.NewCategoryRepo(pg),
		Recommendations: repo.NewRecommendationRepo(pg),
		Producer:        producer,
		Redis:           rdb,
		Cache:           rds.NewCache(rdb, "tv"),
		Streaks:         streaks,
		Leaderboard:     rds.NewLeaderboard(rdb),
		RecsCache:       rds.NewRecommendations(rdb),
		Recommender:     recommender.New(cfg.Recommender.BaseURL, cfg.Recommender.Timeout),
		VisitCh:         visitCh,
	}

	r := api.NewRouter(deps)
	srv := &http.Server{
		Addr:              ":" + cfg.API.Port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		slog.Info("api listening", "port", cfg.API.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("serve", "err", err)
			cancel()
		}
	}()

	<-ctx.Done()
	slog.Info("api shutting down")
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		slog.Error("shutdown", "err", err)
	}
	close(visitCh)
}
