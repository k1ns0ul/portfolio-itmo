package clickhouse

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/andrey/wallet-scoring/internal/models"
)

type WalletRepo struct {
	c *Client
}

func NewWalletRepo(c *Client) *WalletRepo { return &WalletRepo{c: c} }

func (r *WalletRepo) UpsertStats(ctx context.Context, stats []models.WalletStats) error {
	if len(stats) == 0 {
		return nil
	}
	start := time.Now()
	batch, err := r.c.conn.PrepareBatch(ctx, "INSERT INTO wallets.wallet_stats")
	if err != nil {
		r.c.recordLatency(start, err)
		return fmt.Errorf("prepare batch: %w", err)
	}
	for _, s := range stats {
		if !s.Category.Valid() {
			s.Category = models.CategoryUnknown
		}
		if err := batch.Append(
			s.Wallet, s.TxCount, s.FirstSeen, s.LastSeen, s.UniqueCounterparties,
			s.AvgTxAmount, s.MedianTxAmount, s.HerfindahlIndex, s.SmartContractRatio,
			s.VelocityPerHour, s.DormancyDays,
			s.Score, string(s.Category), s.UpdatedAt,
		); err != nil {
			r.c.recordLatency(start, err)
			return fmt.Errorf("append: %w", err)
		}
	}
	err = batch.Send()
	r.c.recordLatency(start, err)
	return err
}

func (r *WalletRepo) GetStats(ctx context.Context, addr string) (*models.WalletStats, error) {
	const q = `
        SELECT wallet, tx_count, first_seen, last_seen, unique_counterparties,
               avg_tx_amount, median_tx_amount, herfindahl_index, smart_contract_ratio,
               velocity_per_hour, dormancy_days, score, category, updated_at
        FROM wallets.wallet_stats FINAL
        WHERE wallet = ? LIMIT 1
    `
	start := time.Now()
	var s models.WalletStats
	var cat string
	err := r.c.conn.QueryRow(ctx, q, addr).Scan(
		&s.Wallet, &s.TxCount, &s.FirstSeen, &s.LastSeen, &s.UniqueCounterparties,
		&s.AvgTxAmount, &s.MedianTxAmount, &s.HerfindahlIndex, &s.SmartContractRatio,
		&s.VelocityPerHour, &s.DormancyDays, &s.Score, &cat, &s.UpdatedAt,
	)
	r.c.recordLatency(start, err)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	s.Category = models.Category(cat)
	return &s, nil
}

func (r *WalletRepo) GetTopByScore(ctx context.Context, limit, offset uint32) ([]models.WalletStats, error) {
	if limit == 0 || limit > 500 {
		limit = 50
	}
	const q = `
        SELECT wallet, tx_count, first_seen, last_seen, unique_counterparties,
               avg_tx_amount, median_tx_amount, herfindahl_index, smart_contract_ratio,
               velocity_per_hour, dormancy_days, score, category, updated_at
        FROM wallets.wallet_stats FINAL
        ORDER BY score DESC, wallet ASC
        LIMIT ? OFFSET ?
    `
	start := time.Now()
	rows, err := r.c.conn.Query(ctx, q, limit, offset)
	if err != nil {
		r.c.recordLatency(start, err)
		return nil, err
	}
	defer rows.Close()

	out := make([]models.WalletStats, 0, limit)
	for rows.Next() {
		var s models.WalletStats
		var cat string
		if err := rows.Scan(
			&s.Wallet, &s.TxCount, &s.FirstSeen, &s.LastSeen, &s.UniqueCounterparties,
			&s.AvgTxAmount, &s.MedianTxAmount, &s.HerfindahlIndex, &s.SmartContractRatio,
			&s.VelocityPerHour, &s.DormancyDays, &s.Score, &cat, &s.UpdatedAt,
		); err != nil {
			r.c.recordLatency(start, err)
			return nil, err
		}
		s.Category = models.Category(cat)
		out = append(out, s)
	}
	r.c.recordLatency(start, rows.Err())
	return out, rows.Err()
}

func (r *WalletRepo) GetScoreDistribution(ctx context.Context) (models.ScoreDistribution, error) {
	const q = `
        SELECT
            floor(score / 10) * 10 AS bucket,
            count() AS cnt
        FROM wallets.wallet_stats FINAL
        GROUP BY bucket
        ORDER BY bucket
    `
	start := time.Now()
	rows, err := r.c.conn.Query(ctx, q)
	if err != nil {
		r.c.recordLatency(start, err)
		return models.ScoreDistribution{}, err
	}
	defer rows.Close()
	var out models.ScoreDistribution
	for rows.Next() {
		var bucket float64
		var cnt uint64
		if err := rows.Scan(&bucket, &cnt); err != nil {
			r.c.recordLatency(start, err)
			return out, err
		}
		out.Buckets = append(out.Buckets, models.ScoreBucket{
			From:  float32(bucket),
			To:    float32(bucket + 10),
			Count: cnt,
		})
	}
	r.c.recordLatency(start, rows.Err())
	return out, rows.Err()
}

func (r *WalletRepo) Search(ctx context.Context, prefix string, limit int) ([]models.WalletStats, error) {
	if prefix == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	const q = `
        SELECT wallet, tx_count, first_seen, last_seen, unique_counterparties,
               avg_tx_amount, median_tx_amount, herfindahl_index, smart_contract_ratio,
               velocity_per_hour, dormancy_days, score, category, updated_at
        FROM wallets.wallet_stats FINAL
        WHERE startsWith(wallet, ?)
        ORDER BY score DESC
        LIMIT ?
    `
	start := time.Now()
	rows, err := r.c.conn.Query(ctx, q, prefix, limit)
	if err != nil {
		r.c.recordLatency(start, err)
		return nil, err
	}
	defer rows.Close()
	out := make([]models.WalletStats, 0, limit)
	for rows.Next() {
		var s models.WalletStats
		var cat string
		if err := rows.Scan(
			&s.Wallet, &s.TxCount, &s.FirstSeen, &s.LastSeen, &s.UniqueCounterparties,
			&s.AvgTxAmount, &s.MedianTxAmount, &s.HerfindahlIndex, &s.SmartContractRatio,
			&s.VelocityPerHour, &s.DormancyDays, &s.Score, &cat, &s.UpdatedAt,
		); err != nil {
			r.c.recordLatency(start, err)
			return nil, err
		}
		s.Category = models.Category(cat)
		out = append(out, s)
	}
	r.c.recordLatency(start, rows.Err())
	return out, rows.Err()
}

func (r *WalletRepo) RecordHistory(ctx context.Context, points []models.WalletScore) error {
	if len(points) == 0 {
		return nil
	}
	start := time.Now()
	batch, err := r.c.conn.PrepareBatch(ctx, "INSERT INTO wallets.score_history")
	if err != nil {
		r.c.recordLatency(start, err)
		return err
	}
	for _, p := range points {
		if err := batch.Append(p.Wallet, p.Score, string(p.Category), p.UpdatedAt); err != nil {
			r.c.recordLatency(start, err)
			return err
		}
	}
	err = batch.Send()
	r.c.recordLatency(start, err)
	return err
}

func (r *WalletRepo) GetHistory(ctx context.Context, addr string, from time.Time, limit int) ([]models.WalletScore, error) {
	if limit <= 0 || limit > 5000 {
		limit = 500
	}
	const q = `
        SELECT wallet, score, category, ts
        FROM wallets.score_history
        WHERE wallet = ? AND ts >= ?
        ORDER BY ts ASC
        LIMIT ?
    `
	start := time.Now()
	rows, err := r.c.conn.Query(ctx, q, addr, from, limit)
	if err != nil {
		r.c.recordLatency(start, err)
		return nil, err
	}
	defer rows.Close()
	out := make([]models.WalletScore, 0, limit)
	for rows.Next() {
		var p models.WalletScore
		var cat string
		if err := rows.Scan(&p.Wallet, &p.Score, &cat, &p.UpdatedAt); err != nil {
			r.c.recordLatency(start, err)
			return nil, err
		}
		p.Category = models.Category(cat)
		out = append(out, p)
	}
	r.c.recordLatency(start, rows.Err())
	return out, rows.Err()
}

func (r *WalletRepo) GlobalCount(ctx context.Context) (uint64, error) {
	var n uint64
	start := time.Now()
	err := r.c.conn.QueryRow(ctx, "SELECT count() FROM wallets.wallet_stats FINAL").Scan(&n)
	r.c.recordLatency(start, err)
	return n, err
}
