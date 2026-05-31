package main

import (
	"context"
	"log/slog"
	"net"
	"os"

	"github.com/andrey/mpc-cluster/internal/common"
	"github.com/andrey/mpc-cluster/internal/config"
	mgrpc "github.com/andrey/mpc-cluster/internal/grpc"
	"github.com/andrey/mpc-cluster/internal/node"
	"google.golang.org/grpc"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(log); err != nil {
		log.Error("node exited", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.LoadNode()
	if err != nil {
		return err
	}
	log = log.With("node", cfg.NodeID)

	lis, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return err
	}

	buf := node.NewBuffer()
	peerSrv := grpc.NewServer()
	node.NewPeerService(buf, log).Register(peerSrv)

	go func() {
		log.Info("peer server listening", "addr", cfg.ListenAddr)
		if err := peerSrv.Serve(lis); err != nil {
			log.Error("peer server stopped", "err", err)
		}
	}()

	coord, err := mgrpc.DialCoordinator(cfg.CoordinatorAddr)
	if err != nil {
		return err
	}
	defer coord.Close()

	advertise := cfg.Advertise
	if advertise == "" {
		advertise = cfg.ListenAddr
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		common.WaitForShutdown(ctx)
		cancel()
	}()

	worker := node.NewWorker(cfg, advertise, coord, buf, log)
	runErr := worker.Run(ctx)

	peerSrv.GracefulStop()
	if runErr != nil {
		return runErr
	}
	log.Info("node finished")
	return nil
}
