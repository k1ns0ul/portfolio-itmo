package config

import (
	"flag"
	"fmt"
	"strings"
)

type NodeConfig struct {
	ID        string
	HTTPAddr  string
	RaftAddr  string
	GRPCAddr  string
	DataDir   string
	Peers     []string
	Bootstrap bool
}

func FromFlags(args []string) (*NodeConfig, error) {
	fs := flag.NewFlagSet("kv-node", flag.ContinueOnError)

	id := fs.String("id", "", "unique node id")
	httpAddr := fs.String("addr", "0.0.0.0:8080", "http listen address")
	raftAddr := fs.String("raft-addr", "0.0.0.0:7000", "raft transport address")
	grpcAddr := fs.String("grpc-addr", "0.0.0.0:7100", "grpc forward address")
	dataDir := fs.String("data-dir", "/tmp/kv", "data directory for raft state")
	peers := fs.String("peers", "", "comma separated raft peer addresses")
	bootstrap := fs.Bool("bootstrap", false, "bootstrap a new cluster from this node")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("parse flags: %w", err)
	}
	if *id == "" {
		return nil, fmt.Errorf("--id is required")
	}

	cfg := &NodeConfig{
		ID:        *id,
		HTTPAddr:  *httpAddr,
		RaftAddr:  *raftAddr,
		GRPCAddr:  *grpcAddr,
		DataDir:   *dataDir,
		Bootstrap: *bootstrap,
	}
	for _, p := range strings.Split(*peers, ",") {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			cfg.Peers = append(cfg.Peers, trimmed)
		}
	}
	return cfg, nil
}
