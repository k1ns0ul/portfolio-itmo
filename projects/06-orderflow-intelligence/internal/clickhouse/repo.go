package clickhouse

import (
	"context"
	"fmt"
	"time"

	"github.com/andrey/orderflow-intelligence/internal/models"
)

type Repo struct {
	c *Client
}

func NewRepo(c *Client) *Repo { return &Repo{c: c} }

func (r *Repo) InsertSwaps(ctx context.Context, swaps []models.SwapEvent) error {
	if len(swaps) == 0 {
		return nil
	}
	batch, err := r.c.conn.PrepareBatch(ctx, "INSERT INTO orderflow.swaps")
	if err != nil {
		return fmt.Errorf("prepare swaps batch: %w", err)
	}
	for _, s := range swaps {
		if err := batch.Append(
			s.Signature, s.Slot, s.BlockTime, s.Dex, s.PoolAddress,
			s.Pair, s.TokenIn, s.TokenOut, s.AmountIn, s.AmountOut,
			s.Price, string(s.Direction), s.Sender,
		); err != nil {
			return fmt.Errorf("append swap: %w", err)
		}
	}
	return batch.Send()
}

func (r *Repo) InsertWindows(ctx context.Context, windows []models.FeatureWindow) error {
	if len(windows) == 0 {
		return nil
	}
	batch, err := r.c.conn.PrepareBatch(ctx, "INSERT INTO orderflow.feature_windows")
	if err != nil {
		return fmt.Errorf("prepare windows batch: %w", err)
	}
	for _, w := range windows {
		if err := batch.Append(
			w.Pair, uint32(w.IntervalSec), w.WindowStart, w.WindowEnd,
			w.OFI, w.VPIN, w.PriceImpact, w.AvgSwapSize, w.BuyRatio,
			w.CumulativeVolume, w.PriceRange, w.PriceOpen, w.PriceClose,
			uint32(w.SwapCount),
		); err != nil {
			return fmt.Errorf("append window: %w", err)
		}
	}
	return batch.Send()
}

func (r *Repo) GetLatestByPair(ctx context.Context, pair string, intervalSec int, limit int) ([]models.FeatureWindow, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	const q = `
        SELECT pair, interval_sec, window_start, window_end,
               ofi, vpin, price_impact, avg_swap_size, buy_ratio,
               cumulative_volume, price_range, price_open, price_close, swap_count
        FROM orderflow.feature_windows
        WHERE pair = ? AND interval_sec = ?
        ORDER BY window_end DESC LIMIT ?
    `
	rows, err := r.c.conn.Query(ctx, q, pair, uint32(intervalSec), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.FeatureWindow, 0, limit)
	for rows.Next() {
		var w models.FeatureWindow
		var iv uint32
		var swapCount uint32
		if err := rows.Scan(
			&w.Pair, &iv, &w.WindowStart, &w.WindowEnd,
			&w.OFI, &w.VPIN, &w.PriceImpact, &w.AvgSwapSize, &w.BuyRatio,
			&w.CumulativeVolume, &w.PriceRange, &w.PriceOpen, &w.PriceClose, &swapCount,
		); err != nil {
			return nil, err
		}
		w.IntervalSec = int(iv)
		w.SwapCount = int(swapCount)
		out = append(out, w)
	}
	return out, rows.Err()
}

func (r *Repo) InsertPredictions(ctx context.Context, preds []models.Prediction) error {
	if len(preds) == 0 {
		return nil
	}
	batch, err := r.c.conn.PrepareBatch(ctx, "INSERT INTO orderflow.predictions")
	if err != nil {
		return fmt.Errorf("prepare predictions: %w", err)
	}
	now := time.Now().UTC()
	for _, p := range preds {
		createdAt := p.CreatedAt
		if createdAt.IsZero() {
			createdAt = now
		}
		if err := batch.Append(p.Pair, p.WindowEnd, string(p.Direction), p.Confidence, p.XGBProb, p.LSTMProb, createdAt); err != nil {
			return fmt.Errorf("append pred: %w", err)
		}
	}
	return batch.Send()
}

func (r *Repo) GetPredictions(ctx context.Context, pair string, limit int) ([]models.Prediction, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	const q = `
        SELECT pair, window_end, direction, confidence, xgb_prob, lstm_prob, created_at
        FROM orderflow.predictions
        WHERE pair = ?
        ORDER BY window_end DESC LIMIT ?
    `
	rows, err := r.c.conn.Query(ctx, q, pair, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.Prediction, 0, limit)
	for rows.Next() {
		var p models.Prediction
		var dir string
		if err := rows.Scan(&p.Pair, &p.WindowEnd, &dir, &p.Confidence, &p.XGBProb, &p.LSTMProb, &p.CreatedAt); err != nil {
			return nil, err
		}
		p.Direction = models.Direction(dir)
		out = append(out, p)
	}
	return out, rows.Err()
}

type PairStats struct {
	Pair        string  `json:"pair"`
	IntervalSec int     `json:"interval_sec"`
	LastWindow  time.Time `json:"last_window"`
	OFI         float64 `json:"ofi"`
	VPIN        float64 `json:"vpin"`
	PriceClose  float64 `json:"price_close"`
}

func (r *Repo) GetPairStats(ctx context.Context, intervalSec int) ([]PairStats, error) {
	const q = `
        WITH ranked AS (
            SELECT pair, interval_sec, window_end, ofi, vpin, price_close,
                   row_number() OVER (PARTITION BY pair, interval_sec ORDER BY window_end DESC) AS rn
            FROM orderflow.feature_windows
            WHERE interval_sec = ?
        )
        SELECT pair, interval_sec, window_end, ofi, vpin, price_close
        FROM ranked WHERE rn = 1
    `
	rows, err := r.c.conn.Query(ctx, q, uint32(intervalSec))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PairStats, 0, 16)
	for rows.Next() {
		var s PairStats
		var iv uint32
		if err := rows.Scan(&s.Pair, &iv, &s.LastWindow, &s.OFI, &s.VPIN, &s.PriceClose); err != nil {
			return nil, err
		}
		s.IntervalSec = int(iv)
		out = append(out, s)
	}
	return out, rows.Err()
}
