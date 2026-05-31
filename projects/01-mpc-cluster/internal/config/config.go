package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type CoordinatorConfig struct {
	HTTPAddr     string
	GRPCAddr     string
	PostgresDSN  string
	RedisAddr    string
	RedisPass    string
	K8sEnabled   bool
	Namespace    string
	NodeImage    string
	GRPCEndpoint string
	LogLevel     string
}

type NodeConfig struct {
	NodeID          int
	SessionID       string
	CoordinatorAddr string
	ListenAddr      string
	Advertise       string
	Peers           map[int]string
	GatherTimeout   time.Duration
	LogLevel        string
}

func LoadCoordinator() (*CoordinatorConfig, error) {
	c := &CoordinatorConfig{
		HTTPAddr:     getenv("HTTP_ADDR", ":8080"),
		GRPCAddr:     getenv("GRPC_ADDR", ":9090"),
		PostgresDSN:  getenv("POSTGRES_DSN", "postgres://mpc:mpc@localhost:5432/mpc?sslmode=disable"),
		RedisAddr:    getenv("REDIS_ADDR", "localhost:6379"),
		RedisPass:    os.Getenv("REDIS_PASSWORD"),
		Namespace:    getenv("K8S_NAMESPACE", "default"),
		NodeImage:    getenv("NODE_IMAGE", "mpc-cluster/node:latest"),
		GRPCEndpoint: getenv("COORDINATOR_GRPC_ENDPOINT", "mpc-coordinator:9090"),
		LogLevel:     getenv("LOG_LEVEL", "info"),
	}
	c.K8sEnabled = getbool("K8S_ENABLED", false)
	return c, nil
}

func LoadNode() (*NodeConfig, error) {
	idRaw := os.Getenv("NODE_ID")
	id, err := strconv.Atoi(idRaw)
	if err != nil {
		return nil, fmt.Errorf("NODE_ID must be an integer, got %q", idRaw)
	}
	coord := os.Getenv("COORDINATOR_ADDR")
	if coord == "" {
		return nil, fmt.Errorf("COORDINATOR_ADDR is required")
	}
	peers, err := parsePeers(os.Getenv("PEER_ADDRS"))
	if err != nil {
		return nil, err
	}
	to, err := time.ParseDuration(getenv("GATHER_TIMEOUT", "30s"))
	if err != nil {
		return nil, fmt.Errorf("parse GATHER_TIMEOUT: %w", err)
	}
	listen := getenv("LISTEN_ADDR", ":9091")
	advertise := os.Getenv("ADVERTISE_ADDR")
	if advertise == "" {
		if podIP := os.Getenv("POD_IP"); podIP != "" {
			advertise = podIP + portOf(listen)
		}
	}
	return &NodeConfig{
		NodeID:          id,
		SessionID:       os.Getenv("SESSION_ID"),
		CoordinatorAddr: coord,
		ListenAddr:      listen,
		Advertise:       advertise,
		Peers:           peers,
		GatherTimeout:   to,
		LogLevel:        getenv("LOG_LEVEL", "info"),
	}, nil
}

func portOf(addr string) string {
	i := strings.LastIndex(addr, ":")
	if i < 0 {
		return ":9091"
	}
	return addr[i:]
}

func parsePeers(raw string) (map[int]string, error) {
	out := map[int]string{}
	if strings.TrimSpace(raw) == "" {
		return out, nil
	}
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		kv := strings.SplitN(item, "=", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("malformed peer entry %q, want id=addr", item)
		}
		idx, err := strconv.Atoi(strings.TrimSpace(kv[0]))
		if err != nil {
			return nil, fmt.Errorf("peer index %q: %w", kv[0], err)
		}
		out[idx] = strings.TrimSpace(kv[1])
	}
	return out, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getbool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}
