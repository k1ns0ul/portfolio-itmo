package redis

import (
	"context"
	"fmt"
	"time"
)

var tokenBucketScript = `
local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local refill = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local requested = tonumber(ARGV[4])

local data = redis.call("HMGET", key, "tokens", "ts")
local tokens = tonumber(data[1])
local ts = tonumber(data[2])
if tokens == nil then
	tokens = capacity
	ts = now
end

local elapsed = math.max(0, now - ts)
tokens = math.min(capacity, tokens + elapsed * refill)

local allowed = 0
if tokens >= requested then
	allowed = 1
	tokens = tokens - requested
end

redis.call("HMSET", key, "tokens", tokens, "ts", now)
redis.call("EXPIRE", key, 120)
return allowed
`

type RateLimiter struct {
	client   *Client
	capacity int
	refill   float64
}

func NewRateLimiter(c *Client, perMinute int) *RateLimiter {
	return &RateLimiter{
		client:   c,
		capacity: perMinute,
		refill:   float64(perMinute) / 60.0,
	}
}

func (rl *RateLimiter) Allow(ctx context.Context, subject string) (bool, error) {
	now := time.Now().Unix()
	key := "ratelimit:" + subject
	res, err := rl.client.rdb.Eval(ctx, tokenBucketScript, []string{key},
		rl.capacity, rl.refill, now, 1).Int()
	if err != nil {
		return false, fmt.Errorf("rate limit check for %s: %w", subject, err)
	}
	return res == 1, nil
}
