package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	"github.com/andrey/wallet-scoring/internal/aggregator"
	"github.com/andrey/wallet-scoring/internal/clickhouse"
	"github.com/andrey/wallet-scoring/internal/common"
	"github.com/andrey/wallet-scoring/internal/config"
	agg "github.com/andrey/wallet-scoring/internal/grpcint"
	"github.com/andrey/wallet-scoring/internal/kafka"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load("aggregator")
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

	txRepo := clickhouse.NewTxRepo(ch)
	wRepo := clickhouse.NewWalletRepo(ch)

	producer, err := kafka.NewProducer(cfg.Kafka.Brokers)
	if err != nil {
		slog.Error("kafka producer", "err", err)
		os.Exit(1)
	}
	defer func() {
		closeCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		_ = producer.Close(closeCtx)
	}()

	svc := aggregator.New(cfg.Aggregator, txRepo, wRepo, producer, cfg.Kafka.TopicScoreUpdates)

	lis, err := net.Listen("tcp", ":"+cfg.Aggregator.GRPCPort)
	if err != nil {
		slog.Error("listen", "err", err)
		os.Exit(1)
	}

	gs := grpc.NewServer(
		grpc.KeepaliveParams(keepalive.ServerParameters{Time: 30 * time.Second, Timeout: 10 * time.Second}),
		grpc.ForceServerCodec(common.Codec{}),
	)
	agg.RegisterServer(gs, svc)

	go func() {
		slog.Info("aggregator gRPC", "port", cfg.Aggregator.GRPCPort)
		if err := gs.Serve(lis); err != nil {
			slog.Error("grpc serve", "err", err)
			cancel()
		}
	}()

	runErr := make(chan error, 1)
	go func() { runErr <- svc.Run(ctx) }()

	select {
	case <-ctx.Done():
	case err := <-runErr:
		if err != nil {
			slog.Error("service", "err", err)
		}
	}

	gs.GracefulStop()
	slog.Info("aggregator stopped")
}
