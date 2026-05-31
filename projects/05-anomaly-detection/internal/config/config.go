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

	API       APIConfig
	Extractor ExtractorConfig
	Simulator SimulatorConfig
}

type KafkaConfig struct {
	Brokers          []string
	TopicTx          string
	TopicFeatures    string
	TopicAlerts      string
	TopicDLQ         string
	GroupExtractor   string
	GroupMLWorker    string
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

type APIConfig struct {
	Port string
}

type ExtractorConfig struct {
	WindowShort time.Duration
	WindowLong  time.Duration
	BatchSize   int
}

type SimulatorConfig struct {
	TPS            int
	ClientPool     int
	CounterpartyPool int
	SuspiciousRatio float64
	FraudRatio      float64
	DurationSec     int
}

func Load(service string) (Config, error) {
	cfg := Config{
		Env: env("ENV", "dev"),
		Kafka: KafkaConfig{
			Brokers:        splitCSV(env("KAFKA_BROKERS", "localhost:9092")),
			TopicTx:        env("KAFKA_TOPIC_TX", "transactions"),
			TopicFeatures:  env("KAFKA_TOPIC_FEATURES", "tx-features"),
			TopicAlerts:    env("KAFKA_TOPIC_ALERTS", "anomaly-alerts"),
			TopicDLQ:       env("KAFKA_TOPIC_DLQ", "events.dlq"),
			GroupExtractor: env("KAFKA_GROUP_EXTRACTOR", "feature-extractor"),
			GroupMLWorker:  env("KAFKA_GROUP_ML", "ml-worker"),
		},
		Redis: RedisConfig{
			Addr:     env("REDIS_ADDR", "localhost:6379"),
			Password: env("REDIS_PASSWORD", ""),
			DB:       envInt("REDIS_DB", 0),
		},
		ClickHouse: ClickHouseConfig{
			DSN:     env("CLICKHOUSE_DSN", "clickhouse://default@localhost:9000/anomalies"),
			Migrate: envBool("CLICKHOUSE_MIGRATE", true),
		},
		API: APIConfig{
			Port: env("API_PORT", "8080"),
		},
		Extractor: ExtractorConfig{
			WindowShort: envDuration("WINDOW_SHORT", time.Hour),
			WindowLong:  envDuration("WINDOW_LONG", 24*time.Hour),
			BatchSize:   envInt("EXTRACTOR_BATCH", 256),
		},
		Simulator: SimulatorConfig{
			TPS:              envInt("SIM_TPS", 50),
			ClientPool:       envInt("SIM_CLIENTS", 1000),
			CounterpartyPool: envInt("SIM_COUNTERPARTIES", 200),
			SuspiciousRatio:  envFloat("SIM_SUSPICIOUS", 0.10),
			FraudRatio:       envFloat("SIM_FRAUD", 0.05),
			DurationSec:      envInt("SIM_DURATION", 0),
		},
	}
	if err := cfg.validate(service); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (c Config) validate(service string) error {
	var missing []string
	if len(c.Kafka.Brokers) == 0 {
		missing = append(missing, "KAFKA_BROKERS")
	}
	switch service {
	case "extractor":
		if c.Redis.Addr == "" {
			missing = append(missing, "REDIS_ADDR")
		}
	case "api":
		if c.ClickHouse.DSN == "" {
			missing = append(missing, "CLICKHOUSE_DSN")
		}
	case "simulator":
	default:
		return fmt.Errorf("unknown service %q", service)
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing env: %s", strings.Join(missing, ", "))
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

func envFloat(k string, d float64) float64 {
	if v := os.Getenv(k); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
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

func envDuration(k string, d time.Duration) time.Duration {
	if v := os.Getenv(k); v != "" {
		if dur, err := time.ParseDuration(v); err == nil {
			return dur
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
