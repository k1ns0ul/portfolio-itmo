package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/andrey/cfa-bonds/internal/common"
	"github.com/andrey/cfa-bonds/internal/config"
	"github.com/andrey/cfa-bonds/internal/db"
	"github.com/andrey/cfa-bonds/internal/metrics"
	"github.com/andrey/cfa-bonds/internal/repo"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(log); err != nil {
		log.Error("metrics exporter terminated", "err", err)
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

	collector := metrics.NewCollector(metrics.CollectorDeps{
		Issues:    repo.NewIssueRepo(pool),
		Trades:    repo.NewTradeRepo(pool),
		Investors: repo.NewInvestorRepo(pool),
		Coupons:   repo.NewCouponRepo(pool),
		Events:    repo.NewEventRepo(pool),
		Log:       log,
	})

	exporter := metrics.NewExporter(cfg.Workers.MetricsAddr, log, collector)
	errCh := make(chan error, 1)
	go func() { errCh <- exporter.Start() }()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutCancel()
	return exporter.Shutdown(shutCtx)
}
