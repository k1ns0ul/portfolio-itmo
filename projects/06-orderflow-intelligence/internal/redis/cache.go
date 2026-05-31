package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/andrey/orderflow-intelligence/internal/models"
)

var ErrCacheMiss = errors.New("cache miss")

type Cache struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewCache(rdb *redis.Client, ttl time.Duration) *Cache {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &Cache{rdb: rdb, ttl: ttl}
}

func windowKey(pair string, intervalSec int) string {
	return "latest:" + pair + ":" + strconv.Itoa(intervalSec)
}

func predictionKey(pair string) string { return "prediction:" + pair }

func (c *Cache) SetLatestWindow(ctx context.Context, w models.FeatureWindow) error {
	b, err := json.Marshal(w)
	if err != nil {
		return fmt.Errorf("encode window: %w", err)
	}
	return c.rdb.Set(ctx, windowKey(w.Pair, w.IntervalSec), b, c.ttl).Err()
}

func (c *Cache) GetLatestWindow(ctx context.Context, pair string, intervalSec int) (*models.FeatureWindow, error) {
	raw, err := c.rdb.Get(ctx, windowKey(pair, intervalSec)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrCacheMiss
		}
		return nil, fmt.Errorf("read window: %w", err)
	}
	var w models.FeatureWindow
	if err := json.Unmarshal(raw, &w); err != nil {
		return nil, fmt.Errorf("decode window: %w", err)
	}
	return &w, nil
}

func (c *Cache) SetPrediction(ctx context.Context, p models.Prediction) error {
	b, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("encode prediction: %w", err)
	}
	return c.rdb.Set(ctx, predictionKey(p.Pair), b, c.ttl).Err()
}

func (c *Cache) GetPrediction(ctx context.Context, pair string) (*models.Prediction, error) {
	raw, err := c.rdb.Get(ctx, predictionKey(pair)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrCacheMiss
		}
		return nil, fmt.Errorf("read prediction: %w", err)
	}
	var p models.Prediction
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("decode prediction: %w", err)
	}
	return &p, nil
}
