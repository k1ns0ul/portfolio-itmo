package redis

import (
	"context"
	"fmt"

	goredis "github.com/redis/go-redis/v9"
)

func (c *Client) Publish(ctx context.Context, channel, payload string) error {
	if err := c.rdb.Publish(ctx, channel, payload).Err(); err != nil {
		return fmt.Errorf("publish to %s: %w", channel, err)
	}
	return nil
}

func (c *Client) Subscribe(ctx context.Context, channel string) (*goredis.PubSub, error) {
	sub := c.rdb.Subscribe(ctx, channel)
	if _, err := sub.Receive(ctx); err != nil {
		sub.Close()
		return nil, fmt.Errorf("subscribe to %s: %w", channel, err)
	}
	return sub, nil
}

func ReadyChannel(sessionID string) string {
	return fmt.Sprintf("session:%s:ready", sessionID)
}
