package redis

import (
	"context"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
)

type LeaderboardName string

const (
	LeaderboardPurchases LeaderboardName = "purchases_month"
	LeaderboardReferrals LeaderboardName = "referrals_month"
	LeaderboardStreaks   LeaderboardName = "streaks"
)

func (n LeaderboardName) Valid() bool {
	switch n {
	case LeaderboardPurchases, LeaderboardReferrals, LeaderboardStreaks:
		return true
	}
	return false
}

type Leaderboard struct {
	rdb *redis.Client
}

func NewLeaderboard(rdb *redis.Client) *Leaderboard { return &Leaderboard{rdb: rdb} }

func (l *Leaderboard) key(name LeaderboardName) string { return "leaderboard:" + string(name) }

type Entry struct {
	UserID int64   `json:"user_id"`
	Score  float64 `json:"score"`
	Rank   int     `json:"rank"`
}

func (l *Leaderboard) Add(ctx context.Context, name LeaderboardName, userID int64, score float64) error {
	if !name.Valid() {
		return fmt.Errorf("invalid leaderboard %q", name)
	}
	z := redis.Z{Score: score, Member: strconv.FormatInt(userID, 10)}
	if err := l.rdb.ZAdd(ctx, l.key(name), z).Err(); err != nil {
		return fmt.Errorf("zadd: %w", err)
	}
	return nil
}

func (l *Leaderboard) IncrBy(ctx context.Context, name LeaderboardName, userID int64, delta float64) error {
	if !name.Valid() {
		return fmt.Errorf("invalid leaderboard %q", name)
	}
	if err := l.rdb.ZIncrBy(ctx, l.key(name), delta, strconv.FormatInt(userID, 10)).Err(); err != nil {
		return fmt.Errorf("zincrby: %w", err)
	}
	return nil
}

func (l *Leaderboard) Refresh(ctx context.Context, name LeaderboardName, data map[int64]float64) error {
	if !name.Valid() {
		return fmt.Errorf("invalid leaderboard %q", name)
	}
	key := l.key(name)
	pipe := l.rdb.TxPipeline()
	pipe.Del(ctx, key)
	members := make([]redis.Z, 0, len(data))
	for uid, score := range data {
		members = append(members, redis.Z{Score: score, Member: strconv.FormatInt(uid, 10)})
	}
	if len(members) > 0 {
		pipe.ZAdd(ctx, key, members...)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("refresh: %w", err)
	}
	return nil
}

func (l *Leaderboard) TopN(ctx context.Context, name LeaderboardName, n int) ([]Entry, error) {
	if !name.Valid() {
		return nil, fmt.Errorf("invalid leaderboard %q", name)
	}
	if n <= 0 || n > 1000 {
		n = 20
	}
	rows, err := l.rdb.ZRevRangeWithScores(ctx, l.key(name), 0, int64(n-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("zrevrange: %w", err)
	}
	out := make([]Entry, 0, len(rows))
	for i, r := range rows {
		uid, _ := strconv.ParseInt(r.Member.(string), 10, 64)
		out = append(out, Entry{UserID: uid, Score: r.Score, Rank: i + 1})
	}
	return out, nil
}

func (l *Leaderboard) RankOf(ctx context.Context, name LeaderboardName, userID int64) (int, error) {
	rank, err := l.rdb.ZRevRank(ctx, l.key(name), strconv.FormatInt(userID, 10)).Result()
	if err != nil {
		return 0, err
	}
	return int(rank) + 1, nil
}
