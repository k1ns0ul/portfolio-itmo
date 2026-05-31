package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/andrey/wallet-scoring/internal/api"
	"github.com/andrey/wallet-scoring/internal/clickhouse"
	"github.com/andrey/wallet-scoring/internal/common"
	"github.com/andrey/wallet-scoring/internal/config"
	agg "github.com/andrey/wallet-scoring/internal/grpcint"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load("api")
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	rootCtx, cancel := common.ShutdownContext()
	defer cancel()

	ch, err := clickhouse.NewClient(rootCtx, cfg.ClickHouse.DSN)
	if err != nil {
		slog.Error("clickhouse", "err", err)
		os.Exit(1)
	}
	defer ch.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer rdb.Close()

	aggClient, err := agg.Dial(rootCtx, cfg.API.AggregatorAddr)
	if err != nil {
		slog.Warn("aggregator dial; degraded mode", "err", err)
	} else {
		defer aggClient.Close()
	}

	r := api.NewRouter(api.Deps{
		Config:     cfg.API,
		TxRepo:     clickhouse.NewTxRepo(ch),
		WalletRepo: clickhouse.NewWalletRepo(ch),
		TokenRepo:  clickhouse.NewTokenRepo(ch),
		Aggregator: aggClient,
		Redis:      rdb,
	})

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

	<-rootCtx.Done()
	slog.Info("api shutting down")

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		slog.Error("shutdown", "err", err)
	}
}
