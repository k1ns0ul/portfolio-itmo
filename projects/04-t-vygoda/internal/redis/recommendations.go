package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/andrey/t-vygoda/internal/models"
)

type Recommendations struct {
	rdb *redis.Client
}

func NewRecommendations(rdb *redis.Client) *Recommendations { return &Recommendations{rdb: rdb} }

func recKey(userID int64) string { return "rec:" + strconv.FormatInt(userID, 10) }

func (r *Recommendations) SetForUser(ctx context.Context, userID int64, items []models.Recommendation, ttl time.Duration) error {
	if ttl == 0 {
		ttl = time.Hour
	}
	b, err := json.Marshal(items)
	if err != nil {
		return fmt.Errorf("encode recs: %w", err)
	}
	if err := r.rdb.Set(ctx, recKey(userID), b, ttl).Err(); err != nil {
		return fmt.Errorf("save recs: %w", err)
	}
	return nil
}

func (r *Recommendations) GetForUser(ctx context.Context, userID int64) ([]models.Recommendation, error) {
	raw, err := r.rdb.Get(ctx, recKey(userID)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("get recs: %w", err)
	}
	var out []models.Recommendation
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode recs: %w", err)
	}
	return out, nil
}

func (r *Recommendations) Invalidate(ctx context.Context, userID int64) error {
	return r.rdb.Del(ctx, recKey(userID)).Err()
}
