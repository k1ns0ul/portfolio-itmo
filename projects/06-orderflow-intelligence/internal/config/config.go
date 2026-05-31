package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env string

	Kafka      KafkaConfig
	Redis      RedisConfig
	ClickHouse ClickHouseConfig

	Geyser    GeyserConfig
	Engine    EngineConfig
	API       APIConfig
	MockGen   MockGenConfig
}

type KafkaConfig struct {
	Brokers       []string
	TopicSwaps    string
	TopicFeatures string
	TopicDLQ      string
	GroupEngine   string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type ClickHouseConfig struct {
	DSN     string
	Migrate bool
}

type GeyserConfig struct {
	Endpoint   string
	Token      string
	Commitment string
}

type EngineConfig struct {
	Intervals []time.Duration
	BatchSize int
}

type APIConfig struct {
	Port string
}

type MockGenConfig struct {
	TPS         int
	DurationSec int
}

func Load(service string) (Config, error) {
	cfg := Config{
		Env: env("ENV", "dev"),
		Kafka: KafkaConfig{
			Brokers:       splitCSV(env("KAFKA_BROKERS", "localhost:9092")),
			TopicSwaps:    env("KAFKA_TOPIC_SWAPS", "swap-events"),
			TopicFeatures: env("KAFKA_TOPIC_FEATURES", "feature-windows"),
			TopicDLQ:      env("KAFKA_TOPIC_DLQ", "events.dlq"),
			GroupEngine:   env("KAFKA_GROUP_ENGINE", "feature-engine"),
		},
		Redis: RedisConfig{
			Addr:     env("REDIS_ADDR", "localhost:6379"),
			Password: env("REDIS_PASSWORD", ""),
			DB:       envInt("REDIS_DB", 0),
		},
		ClickHouse: ClickHouseConfig{
			DSN:     env("CLICKHOUSE_DSN", "clickhouse://default@localhost:9000/orderflow"),
			Migrate: envBool("CLICKHOUSE_MIGRATE", true),
		},
		Geyser: GeyserConfig{
			Endpoint:   env("GRPC_ENDPOINT", ""),
			Token:      env("GRPC_TOKEN", ""),
			Commitment: env("GRPC_COMMITMENT", "confirmed"),
		},
		Engine: EngineConfig{
			Intervals: splitDurations(env("ENGINE_INTERVALS", "1m,5m,15m")),
			BatchSize: envInt("ENGINE_BATCH", 256),
		},
		API: APIConfig{
			Port: env("API_PORT", "8080"),
		},
		MockGen: MockGenConfig{
			TPS:         envInt("MOCK_TPS", 20),
			DurationSec: envInt("MOCK_DURATION", 0),
		},
	}
	if err := cfg.validate(service); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (c Config) validate(service string) error {
	switch service {
	case "parser":
		if c.Geyser.Endpoint == "" {
			return fmt.Errorf("GRPC_ENDPOINT is required for parser")
		}
		if len(c.Kafka.Brokers) == 0 {
			return fmt.Errorf("KAFKA_BROKERS is required")
		}
	case "engine":
		if len(c.Kafka.Brokers) == 0 {
			return fmt.Errorf("KAFKA_BROKERS is required")
		}
		if c.ClickHouse.DSN == "" {
			return fmt.Errorf("CLICKHOUSE_DSN is required")
		}
		if c.Redis.Addr == "" {
			return fmt.Errorf("REDIS_ADDR is required")
		}
	case "api":
		if c.ClickHouse.DSN == "" {
			return fmt.Errorf("CLICKHOUSE_DSN is required")
		}
	case "mockgen":
		if len(c.Kafka.Brokers) == 0 {
			return fmt.Errorf("KAFKA_BROKERS is required")
		}
	default:
		return fmt.Errorf("unknown service %q", service)
	}
	return nil
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func envInt(k string, d int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return d
}

func envBool(k string, d bool) bool {
	if v := os.Getenv(k); v != "" {
		switch strings.ToLower(v) {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}
	return d
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func splitDurations(s string) []time.Duration {
	parts := splitCSV(s)
	out := make([]time.Duration, 0, len(parts))
	for _, p := range parts {
		if d, err := time.ParseDuration(p); err == nil && d > 0 {
			out = append(out, d)
		}
	}
	if len(out) == 0 {
		out = []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute}
	}
	return out
}
