package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env        string
	API        APIConfig
	ClickHouse ClickHouseConfig
	Redis      RedisConfig
	LLM        LLMConfig
	MockData   MockDataConfig
}

type APIConfig struct {
	Port string
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

type LLMConfig struct {
	BaseURL    string
	Timeout    time.Duration
	MaxRetries int
	CacheTTL   time.Duration
}

type MockDataConfig struct {
	Wallets        int
	TokenPool      int
	WhalePct       float64
	TraderPct      float64
	HodlerPct      float64
	DegenPct       float64
}

func Load(service string) (Config, error) {
	cfg := Config{
		Env: env("ENV", "dev"),
		API: APIConfig{Port: env("API_PORT", "8080")},
		ClickHouse: ClickHouseConfig{
			DSN:     env("CLICKHOUSE_DSN", "clickhouse://default@localhost:9000/wallets"),
			Migrate: envBool("CLICKHOUSE_MIGRATE", true),
		},
		Redis: RedisConfig{
			Addr:     env("REDIS_ADDR", "localhost:6379"),
			Password: env("REDIS_PASSWORD", ""),
			DB:       envInt("REDIS_DB", 0),
		},
		LLM: LLMConfig{
			BaseURL:    env("LLM_SERVICE_URL", "http://llm-service:8090"),
			Timeout:    envDuration("LLM_TIMEOUT", 60*time.Second),
			MaxRetries: envInt("LLM_RETRIES", 2),
			CacheTTL:   envDuration("REPORT_CACHE_TTL", time.Hour),
		},
		MockData: MockDataConfig{
			Wallets:   envInt("MOCK_WALLETS", 200),
			TokenPool: envInt("MOCK_TOKEN_POOL", 100),
			WhalePct:  envFloat("MOCK_WHALE", 0.05),
			TraderPct: envFloat("MOCK_TRADER", 0.20),
			HodlerPct: envFloat("MOCK_HODLER", 0.40),
			DegenPct:  envFloat("MOCK_DEGEN", 0.25),
		},
	}
	if err := cfg.validate(service); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (c Config) validate(service string) error {
	switch service {
	case "api", "mockdata":
		if c.ClickHouse.DSN == "" {
			return fmt.Errorf("CLICKHOUSE_DSN is required")
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
