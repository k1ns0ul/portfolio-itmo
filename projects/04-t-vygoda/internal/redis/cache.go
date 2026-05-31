package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrCacheMiss = errors.New("cache miss")

type Cache struct {
	rdb    *redis.Client
	prefix string
}

func NewCache(rdb *redis.Client, prefix string) *Cache {
	return &Cache{rdb: rdb, prefix: prefix}
}

func (c *Cache) key(k string) string {
	if c.prefix == "" {
		return k
	}
	return c.prefix + ":" + k
}

func (c *Cache) Get(ctx context.Context, key string, out any) error {
	raw, err := c.rdb.Get(ctx, c.key(key)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return ErrCacheMiss
		}
		return fmt.Errorf("cache get: %w", err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("cache decode: %w", err)
	}
	return nil
}

func (c *Cache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	b, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("cache encode: %w", err)
	}
	if err := c.rdb.Set(ctx, c.key(key), b, ttl).Err(); err != nil {
		return fmt.Errorf("cache set: %w", err)
	}
	return nil
}

func (c *Cache) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	full := make([]string, len(keys))
	for i, k := range keys {
		full[i] = c.key(k)
	}
	if err := c.rdb.Del(ctx, full...).Err(); err != nil {
		return fmt.Errorf("cache del: %w", err)
	}
	return nil
}

func (c *Cache) GetOrSet(ctx context.Context, key string, ttl time.Duration, out any, load func(context.Context) (any, error)) error {
	if err := c.Get(ctx, key, out); err == nil {
		return nil
	} else if !errors.Is(err, ErrCacheMiss) {
		return err
	}
	val, err := load(ctx)
	if err != nil {
		return err
	}
	if err := c.Set(ctx, key, val, ttl); err != nil {
		return err
	}
	b, _ := json.Marshal(val)
	return json.Unmarshal(b, out)
}
