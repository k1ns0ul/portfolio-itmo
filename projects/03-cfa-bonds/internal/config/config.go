package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DB      DBConfig
	Kafka   KafkaConfig
	Redis   RedisConfig
	API     APIConfig
	Workers WorkersConfig
}

type DBConfig struct {
	DSN          string
	MaxConns     int32
	MinConns     int32
	PingRetries  int
	PingInterval time.Duration
}

type KafkaConfig struct {
	Brokers       []string
	ConsumerGroup string
	ClientID      string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type APIConfig struct {
	ListenAddr      string
	JWTSecret       string
	RateLimitPerMin int
	CORSOrigins     []string
	ShutdownGrace   time.Duration
}

type WorkersConfig struct {
	MetricsAddr   string
	TickInterval  time.Duration
	RunOnce       bool
	BatchSize     int
	SettlementDLQ string
}

func Load() (*Config, error) {
	cfg := &Config{
		DB: DBConfig{
			DSN:          env("DB_DSN", "postgres://cfa:cfa@localhost:5432/cfa?sslmode=disable"),
			MaxConns:     int32(envInt("DB_MAX_CONNS", 20)),
			MinConns:     int32(envInt("DB_MIN_CONNS", 2)),
			PingRetries:  envInt("DB_PING_RETRIES", 10),
			PingInterval: envDuration("DB_PING_INTERVAL", 2*time.Second),
		},
		Kafka: KafkaConfig{
			Brokers:       envList("KAFKA_BROKERS", []string{"localhost:9092"}),
			ConsumerGroup: env("KAFKA_GROUP", "cfa-settlement"),
			ClientID:      env("KAFKA_CLIENT_ID", "cfa-bonds"),
		},
		Redis: RedisConfig{
			Addr:     env("REDIS_ADDR", "localhost:6379"),
			Password: os.Getenv("REDIS_PASSWORD"),
			DB:       envInt("REDIS_DB", 0),
		},
		API: APIConfig{
			ListenAddr:      env("API_ADDR", ":8080"),
			JWTSecret:       env("JWT_SECRET", "dev-secret-change-me"),
			RateLimitPerMin: envInt("RATE_LIMIT_PER_MIN", 200),
			CORSOrigins:     envList("CORS_ORIGINS", []string{"*"}),
			ShutdownGrace:   envDuration("SHUTDOWN_GRACE", 20*time.Second),
		},
		Workers: WorkersConfig{
			MetricsAddr:   env("METRICS_ADDR", ":9090"),
			TickInterval:  envDuration("WORKER_TICK", 1*time.Hour),
			RunOnce:       envBool("RUN_ONCE", false),
			BatchSize:     envInt("WORKER_BATCH", 100),
			SettlementDLQ: env("SETTLEMENT_DLQ", "trade.dlq"),
		},
	}
	if cfg.DB.DSN == "" {
		return nil, fmt.Errorf("DB_DSN must not be empty")
	}
	return cfg, nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envBool(key string, def bool) bool {
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

func envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

func envList(key string, def []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return def
	}
	return out
}
