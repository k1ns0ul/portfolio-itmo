package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env  string
	DB   DBConfig
	Kafka KafkaConfig
	Redis RedisConfig
	JWT   JWTConfig
	API   APIConfig
	Worker WorkerConfig
	Recommender RecommenderConfig
}

type DBConfig struct {
	DSN     string
	Migrate bool
}

type KafkaConfig struct {
	Brokers           []string
	TopicPromos       string
	TopicPurchases    string
	TopicReferrals    string
	TopicCFA          string
	TopicDLQ          string
	GroupPromoWorker  string
	GroupReferral     string
	GroupCFA          string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type JWTConfig struct {
	Secret         string
	AccessTTL      time.Duration
	RefreshTTL     time.Duration
	Issuer         string
}

type APIConfig struct {
	Port      string
	RateLimit int
}

type WorkerConfig struct {
	CFAReconcileInterval time.Duration
	CFANetThreshold      float64
}

type RecommenderConfig struct {
	BaseURL string
	Timeout time.Duration
}

func Load(service string) (Config, error) {
	cfg := Config{
		Env: env("ENV", "dev"),
		DB: DBConfig{
			DSN:     env("PG_DSN", "postgres://app:app@localhost:5432/tvygoda?sslmode=disable"),
			Migrate: envBool("PG_MIGRATE", true),
		},
		Kafka: KafkaConfig{
			Brokers:          splitCSV(env("KAFKA_BROKERS", "localhost:9092")),
			TopicPromos:      env("KAFKA_TOPIC_PROMOS", "promo.events"),
			TopicPurchases:   env("KAFKA_TOPIC_PURCHASES", "purchase.events"),
			TopicReferrals:   env("KAFKA_TOPIC_REFERRALS", "referral.events"),
			TopicCFA:         env("KAFKA_TOPIC_CFA", "cfa.events"),
			TopicDLQ:         env("KAFKA_TOPIC_DLQ", "events.dlq"),
			GroupPromoWorker: env("KAFKA_GROUP_PROMO", "promo-worker"),
			GroupReferral:    env("KAFKA_GROUP_REF", "referral-worker"),
			GroupCFA:         env("KAFKA_GROUP_CFA", "cfa-worker"),
		},
		Redis: RedisConfig{
			Addr:     env("REDIS_ADDR", "localhost:6379"),
			Password: env("REDIS_PASSWORD", ""),
			DB:       envInt("REDIS_DB", 0),
		},
		JWT: JWTConfig{
			Secret:     env("JWT_SECRET", "change-me"),
			AccessTTL:  envDuration("JWT_ACCESS_TTL", 24*time.Hour),
			RefreshTTL: envDuration("JWT_REFRESH_TTL", 30*24*time.Hour),
			Issuer:     env("JWT_ISSUER", "t-vygoda"),
		},
		API: APIConfig{
			Port:      env("API_PORT", "8080"),
			RateLimit: envInt("API_RATE_LIMIT", 100),
		},
		Worker: WorkerConfig{
			CFAReconcileInterval: envDuration("CFA_RECONCILE_INTERVAL", time.Hour),
			CFANetThreshold:      envFloat("CFA_NET_THRESHOLD", 100000),
		},
		Recommender: RecommenderConfig{
			BaseURL: env("RECOMMENDER_URL", "http://recommender:8000"),
			Timeout: envDuration("RECOMMENDER_TIMEOUT", 500*time.Millisecond),
		},
	}
	if err := cfg.validate(service); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (c Config) validate(service string) error {
	var missing []string
	if c.DB.DSN == "" {
		missing = append(missing, "PG_DSN")
	}
	if len(c.Kafka.Brokers) == 0 {
		missing = append(missing, "KAFKA_BROKERS")
	}
	switch service {
	case "api":
		if c.JWT.Secret == "" || c.JWT.Secret == "change-me" && c.Env != "dev" {
			missing = append(missing, "JWT_SECRET")
		}
		if c.Redis.Addr == "" {
			missing = append(missing, "REDIS_ADDR")
		}
	case "promo-worker", "referral-worker", "cfa-worker":
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
