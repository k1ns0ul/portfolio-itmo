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
	ClickHouse ClickHouseConfig
	Redis      RedisConfig

	Ingester   IngesterConfig
	Writer     WriterConfig
	Aggregator AggregatorConfig
	API        APIConfig
	Notifier   NotifierConfig
	MockGen    MockGenConfig
}

type KafkaConfig struct {
	Brokers              []string
	TopicRawTransactions string
	TopicScoreUpdates    string
	TopicAlerts          string
	TopicDLQ             string
	GroupWriter          string
	GroupNotifier        string
}

type ClickHouseConfig struct {
	DSN     string
	Migrate bool
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type IngesterConfig struct {
	GRPCEndpoint string
	GRPCToken    string
	Commitment   string
}

type WriterConfig struct {
	BatchSize     int
	BatchInterval time.Duration
}

type AggregatorConfig struct {
	Interval    time.Duration
	LookbackDur time.Duration
	GRPCPort    string
}

type APIConfig struct {
	Port                  string
	AggregatorAddr        string
	RateLimitPerMinute    int
	APIKeyRequired        bool
	WebsocketWriteTimeout time.Duration
	WebsocketReadTimeout  time.Duration
}

type NotifierConfig struct {
	ScoreThreshold   float32
	WatchListKey     string
	AlertChannel     string
	ScoreUpdateTopic string
}

type MockGenConfig struct {
	TPS            int
	WalletPoolSize int
	ScamRatio      float64
	SuspiciousRate float64
	DurationSec    int
}

func Load(service string) (Config, error) {
	cfg := Config{
		Env: env("ENV", "dev"),
		Kafka: KafkaConfig{
			Brokers:              splitCSV(env("KAFKA_BROKERS", "localhost:9092")),
			TopicRawTransactions: env("KAFKA_TOPIC_TX", "raw-transactions"),
			TopicScoreUpdates:    env("KAFKA_TOPIC_SCORES", "score-updates"),
			TopicAlerts:          env("KAFKA_TOPIC_ALERTS", "alerts"),
			TopicDLQ:             env("KAFKA_TOPIC_DLQ", "raw-transactions-dlq"),
			GroupWriter:          env("KAFKA_GROUP_WRITER", "writer"),
			GroupNotifier:        env("KAFKA_GROUP_NOTIFIER", "notifier"),
		},
		ClickHouse: ClickHouseConfig{
			DSN:     env("CLICKHOUSE_DSN", "clickhouse://default@localhost:9000/wallets"),
			Migrate: envBool("CLICKHOUSE_MIGRATE", true),
		},
		Redis: RedisConfig{
			Addr:     env("REDIS_ADDR", "localhost:6379"),
			Password: env("REDIS_PASSWORD", ""),
			DB:       envInt("REDIS_DB", 0),
		},
		Ingester: IngesterConfig{
			GRPCEndpoint: env("GRPC_ENDPOINT", ""),
			GRPCToken:    env("GRPC_TOKEN", ""),
			Commitment:   env("GRPC_COMMITMENT", "confirmed"),
		},
		Writer: WriterConfig{
			BatchSize:     envInt("WRITER_BATCH_SIZE", 1000),
			BatchInterval: envDuration("WRITER_BATCH_INTERVAL", 5*time.Second),
		},
		Aggregator: AggregatorConfig{
			Interval:    envDuration("AGG_INTERVAL", 30*time.Second),
			LookbackDur: envDuration("AGG_LOOKBACK", 24*time.Hour),
			GRPCPort:    env("AGG_GRPC_PORT", "9090"),
		},
		API: APIConfig{
			Port:                  env("API_PORT", "8080"),
			AggregatorAddr:        env("AGG_GRPC_ADDR", "localhost:9090"),
			RateLimitPerMinute:    envInt("API_RATE_LIMIT", 100),
			APIKeyRequired:        envBool("API_KEY_REQUIRED", false),
			WebsocketWriteTimeout: envDuration("WS_WRITE_TIMEOUT", 10*time.Second),
			WebsocketReadTimeout:  envDuration("WS_READ_TIMEOUT", 60*time.Second),
		},
		Notifier: NotifierConfig{
			ScoreThreshold:   float32(envFloat("NOTIFIER_SCORE_THRESHOLD", 30)),
			WatchListKey:     env("NOTIFIER_WATCH_KEY", "watchlist"),
			AlertChannel:     env("NOTIFIER_ALERT_CHANNEL", "alerts"),
			ScoreUpdateTopic: env("KAFKA_TOPIC_SCORES", "score-updates"),
		},
		MockGen: MockGenConfig{
			TPS:            envInt("MOCK_TPS", 100),
			WalletPoolSize: envInt("MOCK_WALLETS", 5000),
			ScamRatio:      envFloat("MOCK_SCAM_RATIO", 0.05),
			SuspiciousRate: envFloat("MOCK_SUSPICIOUS_RATIO", 0.15),
			DurationSec:    envInt("MOCK_DURATION_SEC", 0),
		},
	}

	if err := cfg.validate(service); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (c Config) validate(service string) error {
	var missing []string
	require := func(name, value string) {
		if value == "" {
			missing = append(missing, name)
		}
	}
	if len(c.Kafka.Brokers) == 0 {
		missing = append(missing, "KAFKA_BROKERS")
	}
	switch service {
	case "ingester":
		require("GRPC_ENDPOINT", c.Ingester.GRPCEndpoint)
	case "writer", "aggregator", "api":
		require("CLICKHOUSE_DSN", c.ClickHouse.DSN)
		require("REDIS_ADDR", c.Redis.Addr)
	case "notifier":
		require("REDIS_ADDR", c.Redis.Addr)
	case "mockgen":
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
