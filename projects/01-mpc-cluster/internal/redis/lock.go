package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

var releaseScript = goredis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
	return redis.call("del", KEYS[1])
else
	return 0
end
`)

type Lock struct {
	client *Client
	key    string
	token  string
}

func (c *Client) Acquire(ctx context.Context, key string, ttl time.Duration) (*Lock, error) {
	token, err := newToken()
	if err != nil {
		return nil, err
	}
	ok, err := c.rdb.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return nil, fmt.Errorf("acquire lock %s: %w", key, err)
	}
	if !ok {
		return nil, fmt.Errorf("lock %s already held", key)
	}
	return &Lock{client: c, key: key, token: token}, nil
}

func (l *Lock) Release(ctx context.Context) error {
	res, err := releaseScript.Run(ctx, l.client.rdb, []string{l.key}, l.token).Int()
	if err != nil {
		return fmt.Errorf("release lock %s: %w", l.key, err)
	}
	if res == 0 {
		return fmt.Errorf("lock %s lost before release", l.key)
	}
	return nil
}

func newToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate lock token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
