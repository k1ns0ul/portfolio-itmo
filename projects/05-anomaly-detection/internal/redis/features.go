package redis

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/andrey/anomaly-detection/internal/models"
)

const (
	keyTTL24h = 25 * time.Hour
	keyTTL1h  = 90 * time.Minute
)

type FeatureStore struct {
	rdb         *redis.Client
	windowShort time.Duration
	windowLong  time.Duration
}

func NewFeatureStore(rdb *redis.Client, windowShort, windowLong time.Duration) *FeatureStore {
	if windowShort <= 0 {
		windowShort = time.Hour
	}
	if windowLong <= 0 {
		windowLong = 24 * time.Hour
	}
	return &FeatureStore{rdb: rdb, windowShort: windowShort, windowLong: windowLong}
}

func amountsKey(c string) string        { return "client:" + c + ":amounts" }
func counterpartiesKey(c string) string { return "client:" + c + ":counterparties:24h" }
func lastTxKey(c string) string         { return "client:" + c + ":last_tx_time" }
func statsKey(c string) string          { return "client:" + c + ":stats" }

func (s *FeatureStore) ComputeFeatures(ctx context.Context, tx models.Transaction) (models.TransactionFeatures, error) {
	now := tx.Timestamp
	if now.IsZero() {
		now = time.Now().UTC()
	}
	ts := float64(now.UnixNano()) / 1e9
	member := tx.ID + "|" + strconv.FormatFloat(tx.Amount, 'g', -1, 64)

	pipe := s.rdb.TxPipeline()
	zaddCmd := pipe.ZAdd(ctx, amountsKey(tx.ClientID), redis.Z{Score: ts, Member: member})
	_ = zaddCmd
	pipe.Expire(ctx, amountsKey(tx.ClientID), keyTTL24h)

	cutoff24h := ts - s.windowLong.Seconds()
	pipe.ZRemRangeByScore(ctx, amountsKey(tx.ClientID), "-inf", fmt.Sprintf("%f", cutoff24h))

	cutoff1h := ts - s.windowShort.Seconds()
	rangeShort := pipe.ZRangeByScoreWithScores(ctx, amountsKey(tx.ClientID),
		&redis.ZRangeBy{Min: fmt.Sprintf("%f", cutoff1h), Max: "+inf"})
	rangeLong := pipe.ZRangeByScoreWithScores(ctx, amountsKey(tx.ClientID),
		&redis.ZRangeBy{Min: fmt.Sprintf("%f", cutoff24h), Max: "+inf"})

	pipe.PFAdd(ctx, counterpartiesKey(tx.ClientID), tx.CounterpartyID)
	pipe.Expire(ctx, counterpartiesKey(tx.ClientID), keyTTL24h)
	uniqueCmd := pipe.PFCount(ctx, counterpartiesKey(tx.ClientID))

	lastTxCmd := pipe.Get(ctx, lastTxKey(tx.ClientID))
	pipe.Set(ctx, lastTxKey(tx.ClientID), strconv.FormatFloat(ts, 'f', -1, 64), keyTTL24h)

	statsCmd := pipe.HGetAll(ctx, statsKey(tx.ClientID))

	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return models.TransactionFeatures{}, fmt.Errorf("pipeline: %w", err)
	}

	shortAmounts := parseAmounts(rangeShort.Val())
	longAmounts := parseAmounts(rangeLong.Val())

	stats := parseStats(statsCmd.Val())
	zScore := computeZScore(tx.Amount, stats)
	newStats := welfordUpdate(stats, tx.Amount)
	if err := s.writeStats(ctx, tx.ClientID, newStats); err != nil {
		return models.TransactionFeatures{}, fmt.Errorf("write stats: %w", err)
	}

	timeSinceLast := 0.0
	if lastTxCmd.Err() == nil {
		if prev, err := strconv.ParseFloat(lastTxCmd.Val(), 64); err == nil {
			timeSinceLast = ts - prev
			if timeSinceLast < 0 {
				timeSinceLast = 0
			}
		}
	}

	nightFlag := 0.0
	if h := now.Hour(); h >= 0 && h < 6 {
		nightFlag = 1.0
	}

	return models.TransactionFeatures{
		TxID:                    tx.ID,
		ClientID:                tx.ClientID,
		Amount:                  tx.Amount,
		AvgAmount1h:             mean(shortAmounts),
		AvgAmount24h:            mean(longAmounts),
		UniqueCounterparties24h: float64(uniqueCmd.Val()),
		ZScore:                  zScore,
		TimeSinceLastTx:         timeSinceLast,
		NightFlag:               nightFlag,
		FrequencyScore:          float64(len(shortAmounts)),
		Timestamp:               now,
	}, nil
}

type clientStats struct {
	Count int64
	Mean  float64
	M2    float64
}

func parseStats(m map[string]string) clientStats {
	out := clientStats{}
	if v, ok := m["count"]; ok {
		out.Count, _ = strconv.ParseInt(v, 10, 64)
	}
	if v, ok := m["mean"]; ok {
		out.Mean, _ = strconv.ParseFloat(v, 64)
	}
	if v, ok := m["m2"]; ok {
		out.M2, _ = strconv.ParseFloat(v, 64)
	}
	return out
}

func welfordUpdate(s clientStats, x float64) clientStats {
	s.Count++
	delta := x - s.Mean
	s.Mean += delta / float64(s.Count)
	delta2 := x - s.Mean
	s.M2 += delta * delta2
	return s
}

func computeZScore(x float64, s clientStats) float64 {
	if s.Count < 2 {
		return 0
	}
	variance := s.M2 / float64(s.Count-1)
	if variance <= 0 {
		return 0
	}
	stddev := math.Sqrt(variance)
	return (x - s.Mean) / stddev
}

func (s *FeatureStore) writeStats(ctx context.Context, clientID string, st clientStats) error {
	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, statsKey(clientID),
		"count", st.Count,
		"mean", strconv.FormatFloat(st.Mean, 'f', -1, 64),
		"m2", strconv.FormatFloat(st.M2, 'f', -1, 64),
	)
	pipe.Expire(ctx, statsKey(clientID), keyTTL24h)
	_, err := pipe.Exec(ctx)
	return err
}

func parseAmounts(zs []redis.Z) []float64 {
	out := make([]float64, 0, len(zs))
	for _, z := range zs {
		member, ok := z.Member.(string)
		if !ok {
			continue
		}
		idx := -1
		for i := 0; i < len(member); i++ {
			if member[i] == '|' {
				idx = i
				break
			}
		}
		if idx < 0 || idx+1 >= len(member) {
			continue
		}
		if amt, err := strconv.ParseFloat(member[idx+1:], 64); err == nil {
			out = append(out, amt)
		}
	}
	return out
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var s float64
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}
