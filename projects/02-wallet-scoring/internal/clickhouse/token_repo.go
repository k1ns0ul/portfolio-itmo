package clickhouse

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/andrey/wallet-scoring/internal/models"
)

type TokenRepo struct {
	c *Client
}

func NewTokenRepo(c *Client) *TokenRepo { return &TokenRepo{c: c} }

func (r *TokenRepo) Upsert(ctx context.Context, scores []models.TokenScore) error {
	if len(scores) == 0 {
		return nil
	}
	start := time.Now()
	batch, err := r.c.conn.PrepareBatch(ctx, "INSERT INTO wallets.token_scores")
	if err != nil {
		r.c.recordLatency(start, err)
		return err
	}
	for _, s := range scores {
		if !s.Category.Valid() {
			s.Category = models.CategoryUnknown
		}
		if err := batch.Append(
			s.Mint, string(s.Category), s.Confidence, s.RiskScore,
			s.Holders, s.Volume24h, s.UpdatedAt,
		); err != nil {
			r.c.recordLatency(start, err)
			return err
		}
	}
	err = batch.Send()
	r.c.recordLatency(start, err)
	return err
}

func (r *TokenRepo) Get(ctx context.Context, mint string) (*models.TokenScore, error) {
	const q = `
        SELECT mint, category, confidence, risk_score, holders, volume_24h, updated_at
        FROM wallets.token_scores FINAL
        WHERE mint = ? LIMIT 1
    `
	start := time.Now()
	var s models.TokenScore
	var cat string
	err := r.c.conn.QueryRow(ctx, q, mint).Scan(
		&s.Mint, &cat, &s.Confidence, &s.RiskScore, &s.Holders, &s.Volume24h, &s.UpdatedAt,
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

func (r *TokenRepo) ListByCategory(ctx context.Context, category models.Category, limit int) ([]models.TokenScore, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	const q = `
        SELECT mint, category, confidence, risk_score, holders, volume_24h, updated_at
        FROM wallets.token_scores FINAL
        WHERE category = ?
        ORDER BY risk_score DESC
        LIMIT ?
    `
	start := time.Now()
	rows, err := r.c.conn.Query(ctx, q, string(category), limit)
	if err != nil {
		r.c.recordLatency(start, err)
		return nil, err
	}
	defer rows.Close()
	out := make([]models.TokenScore, 0, limit)
	for rows.Next() {
		var s models.TokenScore
		var cat string
		if err := rows.Scan(&s.Mint, &cat, &s.Confidence, &s.RiskScore, &s.Holders, &s.Volume24h, &s.UpdatedAt); err != nil {
			r.c.recordLatency(start, err)
			return nil, err
		}
		s.Category = models.Category(cat)
		out = append(out, s)
	}
	r.c.recordLatency(start, rows.Err())
	return out, rows.Err()
}

func (r *TokenRepo) Suspicious(ctx context.Context, limit int) ([]models.TokenScore, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	const q = `
        SELECT mint, category, confidence, risk_score, holders, volume_24h, updated_at
        FROM wallets.token_scores FINAL
        WHERE category IN ('suspicious', 'scam')
        ORDER BY risk_score DESC
        LIMIT ?
    `
	start := time.Now()
	rows, err := r.c.conn.Query(ctx, q, limit)
	if err != nil {
		r.c.recordLatency(start, err)
		return nil, err
	}
	defer rows.Close()
	out := make([]models.TokenScore, 0, limit)
	for rows.Next() {
		var s models.TokenScore
		var cat string
		if err := rows.Scan(&s.Mint, &cat, &s.Confidence, &s.RiskScore, &s.Holders, &s.Volume24h, &s.UpdatedAt); err != nil {
			r.c.recordLatency(start, err)
			return nil, err
		}
		s.Category = models.Category(cat)
		out = append(out, s)
	}
	r.c.recordLatency(start, rows.Err())
	return out, rows.Err()
}
