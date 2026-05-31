package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/andrey/portfolio-reports/internal/clickhouse"
	"github.com/andrey/portfolio-reports/internal/config"
	"github.com/andrey/portfolio-reports/internal/mockdata"
	"github.com/andrey/portfolio-reports/migrations"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load("mockdata")
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

	repo := clickhouse.NewRepo(ch)
	gen := mockdata.New(cfg.MockData, repo)
	if err := gen.Run(ctx); err != nil {
		slog.Error("mockdata", "err", err)
		os.Exit(1)
	}
}
