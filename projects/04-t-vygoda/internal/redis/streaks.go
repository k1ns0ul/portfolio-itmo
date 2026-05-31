package redis

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/andrey/t-vygoda/internal/models"
)

type Streaks struct {
	rdb *redis.Client
}

func NewStreaks(rdb *redis.Client) *Streaks { return &Streaks{rdb: rdb} }

func streakKey(userID int64) string { return "streak:" + strconv.FormatInt(userID, 10) }

func (s *Streaks) RecordVisit(ctx context.Context, userID int64) (models.UserStreak, error) {
	key := streakKey(userID)
	today := time.Now().UTC().Truncate(24 * time.Hour)

	vals, err := s.rdb.HMGet(ctx, key, "current", "longest", "last_visit").Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return models.UserStreak{}, fmt.Errorf("read streak: %w", err)
	}

	current, longest := 0, 0
	var lastVisit time.Time
	if vals[0] != nil {
		current, _ = strconv.Atoi(vals[0].(string))
	}
	if vals[1] != nil {
		longest, _ = strconv.Atoi(vals[1].(string))
	}
	if vals[2] != nil {
		if ts, err := time.Parse(time.RFC3339, vals[2].(string)); err == nil {
			lastVisit = ts
		}
	}

	switch {
	case lastVisit.IsZero():
		current = 1
	case lastVisit.UTC().Truncate(24*time.Hour).Equal(today):
		return models.UserStreak{
			UserID: userID, CurrentStreak: current, LongestStreak: longest, LastVisit: lastVisit,
		}, nil
	case lastVisit.UTC().Truncate(24*time.Hour).Equal(today.Add(-24 * time.Hour)):
		current++
	default:
		current = 1
	}

	if current > longest {
		longest = current
	}

	if err := s.rdb.HSet(ctx, key,
		"current", current,
		"longest", longest,
		"last_visit", today.Format(time.RFC3339),
	).Err(); err != nil {
		return models.UserStreak{}, fmt.Errorf("write streak: %w", err)
	}
	s.rdb.Expire(ctx, key, 90*24*time.Hour)

	return models.UserStreak{
		UserID:        userID,
		CurrentStreak: current,
		LongestStreak: longest,
		LastVisit:     today,
	}, nil
}

func (s *Streaks) Get(ctx context.Context, userID int64) (models.UserStreak, error) {
	key := streakKey(userID)
	vals, err := s.rdb.HMGet(ctx, key, "current", "longest", "last_visit").Result()
	if err != nil {
		return models.UserStreak{}, fmt.Errorf("read streak: %w", err)
	}
	out := models.UserStreak{UserID: userID}
	if vals[0] != nil {
		out.CurrentStreak, _ = strconv.Atoi(vals[0].(string))
	}
	if vals[1] != nil {
		out.LongestStreak, _ = strconv.Atoi(vals[1].(string))
	}
	if vals[2] != nil {
		if ts, err := time.Parse(time.RFC3339, vals[2].(string)); err == nil {
			out.LastVisit = ts
		}
	}
	return out, nil
}
