package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/andrey/mpc-cluster/internal/common"
	"github.com/andrey/mpc-cluster/internal/config"
	"github.com/andrey/mpc-cluster/internal/coordinator"
	"github.com/andrey/mpc-cluster/internal/db"
	"github.com/andrey/mpc-cluster/internal/k8s"
	"github.com/andrey/mpc-cluster/internal/redis"
	"github.com/andrey/mpc-cluster/internal/session"
	"google.golang.org/grpc"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(log); err != nil {
		log.Error("coordinator exited", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.LoadCoordinator()
	if err != nil {
		return err
	}

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := db.Connect(rootCtx, cfg.PostgresDSN)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := db.Migrate(rootCtx, pool); err != nil {
		return err
	}
	log.Info("postgres ready")

	rc, err := redis.New(rootCtx, cfg.RedisAddr, cfg.RedisPass)
	if err != nil {
		return err
	}
	defer rc.Close()
	log.Info("redis ready")

	var kc *k8s.Client
	if cfg.K8sEnabled {
		kc, err = k8s.NewInCluster()
		if err != nil {
			return err
		}
		log.Info("kubernetes client ready", "namespace", cfg.Namespace)
	}

	store := session.NewStore(pool)
	manager := session.NewManager(store, rc, kc, session.ManagerConfig{
		NodeImage:       cfg.NodeImage,
		Namespace:       cfg.Namespace,
		CoordinatorGRPC: cfg.GRPCEndpoint,
		K8sEnabled:      cfg.K8sEnabled,
		ResultTimeout:   2 * time.Minute,
	}, log)

	grpcSrv := grpc.NewServer()
	coordinator.NewGRPCServer(manager, log).Register(grpcSrv)

	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return err
	}

	handlers := coordinator.NewHandlers(manager, store)
	httpSrv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           coordinator.NewRouter(handlers, log),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 2)
	go func() {
		log.Info("grpc listening", "addr", cfg.GRPCAddr)
		errCh <- grpcSrv.Serve(lis)
	}()
	go func() {
		log.Info("http listening", "addr", cfg.HTTPAddr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-waitSignal(rootCtx):
		log.Info("shutdown requested")
	}

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutCancel()
	if err := httpSrv.Shutdown(shutCtx); err != nil {
		log.Warn("http shutdown", "err", err)
	}
	grpcSrv.GracefulStop()
	log.Info("coordinator stopped")
	return nil
}

func waitSignal(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		common.WaitForShutdown(ctx)
		close(done)
	}()
	return done
}
