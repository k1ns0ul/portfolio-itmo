package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/andrey/portfolio-reports/internal/models"
)

var ErrCacheMiss = errors.New("cache miss")

type ReportCache struct {
	rdb *redis.Client
}

func NewReportCache(rdb *redis.Client) *ReportCache {
	return &ReportCache{rdb: rdb}
}

func reportKey(address string) string { return "report:" + address }

func (c *ReportCache) SetReport(ctx context.Context, address string, r *models.Report, ttl time.Duration) error {
	b, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("encode report: %w", err)
	}
	if err := c.rdb.Set(ctx, reportKey(address), b, ttl).Err(); err != nil {
		return fmt.Errorf("set cache: %w", err)
	}
	return nil
}

func (c *ReportCache) GetReport(ctx context.Context, address string) (*models.Report, error) {
	raw, err := c.rdb.Get(ctx, reportKey(address)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrCacheMiss
		}
		return nil, fmt.Errorf("get cache: %w", err)
	}
	var r models.Report
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("decode cache: %w", err)
	}
	return &r, nil
}

func (c *ReportCache) Invalidate(ctx context.Context, address string) error {
	return c.rdb.Del(ctx, reportKey(address)).Err()
}
