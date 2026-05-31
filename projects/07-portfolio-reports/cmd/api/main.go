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

	"github.com/andrey/portfolio-reports/internal/api"
	"github.com/andrey/portfolio-reports/internal/clickhouse"
	"github.com/andrey/portfolio-reports/internal/config"
	"github.com/andrey/portfolio-reports/internal/llm"
	"github.com/andrey/portfolio-reports/internal/metrics"
	rds "github.com/andrey/portfolio-reports/internal/redis"
	"github.com/andrey/portfolio-reports/migrations"
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
	defer func() {
		if err := rdb.Close(); err != nil {
			slog.Warn("redis close", "err", err)
		}
	}()

	llmClient, err := llm.NewClient(cfg.LLM.BaseURL, cfg.LLM.Timeout, cfg.LLM.MaxRetries)
	if err != nil {
		slog.Error("llm client", "err", err)
		os.Exit(1)
	}

	router := api.NewRouter(&api.Deps{
		Cfg:     cfg.LLM,
		Metrics: metrics.New(clickhouse.NewRepo(ch)),
		LLM:     llmClient,
		Cache:   rds.NewReportCache(rdb),
		CH:      ch,
	})

	srv := &http.Server{
		Addr:              ":" + cfg.API.Port,
		Handler:           router,
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
}
