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

	"github.com/andrey/anomaly-detection/internal/api"
	"github.com/andrey/anomaly-detection/internal/clickhouse"
	"github.com/andrey/anomaly-detection/internal/config"
	"github.com/andrey/anomaly-detection/migrations"
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

	r := api.NewRouter(&api.Deps{
		Alerts: clickhouse.NewAlertRepo(ch),
		CH:     ch,
	})

	srv := &http.Server{
		Addr:              ":" + cfg.API.Port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
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
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		slog.Error("shutdown", "err", err)
	}
}
