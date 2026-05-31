package features

import (
	"context"
	"log/slog"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/andrey/orderflow-intelligence/internal/clickhouse"
	"github.com/andrey/orderflow-intelligence/internal/models"
	rds "github.com/andrey/orderflow-intelligence/internal/redis"
)

const vpinBuckets = 20

type Aggregator struct {
	interval time.Duration
	repo     *clickhouse.Repo
	cache    *rds.Cache

	mu    sync.Mutex
	swaps map[string][]models.SwapEvent
}

func NewAggregator(interval time.Duration, repo *clickhouse.Repo, cache *rds.Cache) *Aggregator {
	if interval <= 0 {
		interval = time.Minute
	}
	return &Aggregator{
		interval: interval,
		repo:     repo,
		cache:    cache,
		swaps:    make(map[string][]models.SwapEvent, 32),
	}
}

func (a *Aggregator) Add(s models.SwapEvent) {
	if s.Pair == "" {
		return
	}
	a.mu.Lock()
	a.swaps[s.Pair] = append(a.swaps[s.Pair], s)
	a.mu.Unlock()
}

func (a *Aggregator) Run(ctx context.Context) {
	t := time.NewTicker(a.interval)
	defer t.Stop()
	slog.Info("aggregator started", "interval", a.interval.String())
	for {
		select {
		case <-ctx.Done():
			a.flush(ctx, time.Now().UTC())
			return
		case end := <-t.C:
			a.flush(ctx, end.UTC())
		}
	}
}

func (a *Aggregator) flush(ctx context.Context, end time.Time) {
	a.mu.Lock()
	snapshot := a.swaps
	a.swaps = make(map[string][]models.SwapEvent, len(snapshot))
	a.mu.Unlock()

	if len(snapshot) == 0 {
		return
	}
	windows := make([]models.FeatureWindow, 0, len(snapshot))
	start := end.Add(-a.interval)
	intervalSec := int(a.interval.Seconds())

	for pair, swaps := range snapshot {
		w := computeWindow(pair, swaps, intervalSec, start, end)
		windows = append(windows, w)
		if err := a.cache.SetLatestWindow(ctx, w); err != nil {
			slog.Warn("cache set", "err", err, "pair", pair)
		}
	}

	writeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := a.repo.InsertWindows(writeCtx, windows); err != nil {
		slog.Error("insert windows", "err", err, "count", len(windows))
		return
	}
	slog.Info("windows written", "count", len(windows), "interval_sec", intervalSec)
}

func computeWindow(pair string, swaps []models.SwapEvent, intervalSec int, start, end time.Time) models.FeatureWindow {
	w := models.FeatureWindow{
		Pair:        pair,
		IntervalSec: intervalSec,
		WindowStart: start,
		WindowEnd:   end,
		SwapCount:   len(swaps),
	}
	if len(swaps) == 0 {
		return w
	}

	sort.Slice(swaps, func(i, j int) bool { return swaps[i].BlockTime.Before(swaps[j].BlockTime) })

	var buyVol, sellVol, totalVol float64
	var buyCount int
	prices := make([]float64, 0, len(swaps))
	minPrice, maxPrice := math.MaxFloat64, -math.MaxFloat64

	for _, s := range swaps {
		vol := float64(s.AmountIn)
		totalVol += vol
		if s.IsBuy() {
			buyVol += vol
			buyCount++
		} else {
			sellVol += vol
		}
		if s.Price > 0 {
			prices = append(prices, s.Price)
			if s.Price < minPrice {
				minPrice = s.Price
			}
			if s.Price > maxPrice {
				maxPrice = s.Price
			}
		}
	}

	w.OFI = buyVol - sellVol
	w.AvgSwapSize = totalVol / float64(len(swaps))
	w.BuyRatio = float64(buyCount) / float64(len(swaps))
	w.CumulativeVolume = totalVol
	w.VPIN = computeVPIN(swaps, totalVol)

	if len(prices) > 0 {
		w.PriceOpen = prices[0]
		w.PriceClose = prices[len(prices)-1]
		if w.PriceOpen > 0 {
			w.PriceImpact = (w.PriceClose - w.PriceOpen) / w.PriceOpen
		}
		avg := mean(prices)
		if avg > 0 {
			w.PriceRange = (maxPrice - minPrice) / avg
		}
	}
	return w
}

func computeVPIN(swaps []models.SwapEvent, totalVol float64) float64 {
	if totalVol <= 0 || len(swaps) == 0 {
		return 0
	}
	bucketSize := totalVol / float64(vpinBuckets)
	if bucketSize <= 0 {
		return 0
	}

	var (
		bucketBuy, bucketSell float64
		bucketVol             float64
		acc                   float64
	)
	for _, s := range swaps {
		vol := float64(s.AmountIn)
		remaining := vol
		for remaining > 0 {
			capacity := bucketSize - bucketVol
			take := remaining
			if take > capacity {
				take = capacity
			}
			share := take / vol
			if s.IsBuy() {
				bucketBuy += vol * share
			} else {
				bucketSell += vol * share
			}
			bucketVol += take
			remaining -= take
			if bucketVol >= bucketSize-1e-9 {
				acc += math.Abs(bucketBuy - bucketSell)
				bucketBuy = 0
				bucketSell = 0
				bucketVol = 0
			}
		}
	}
	if bucketVol > 0 {
		acc += math.Abs(bucketBuy - bucketSell)
	}
	return acc / (2.0 * totalVol)
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
