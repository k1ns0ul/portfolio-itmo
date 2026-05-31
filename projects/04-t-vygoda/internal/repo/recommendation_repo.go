package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/andrey/t-vygoda/internal/db"
	"github.com/andrey/t-vygoda/internal/models"
)

type RecommendationRepo struct {
	db *db.DB
}

func NewRecommendationRepo(d *db.DB) *RecommendationRepo { return &RecommendationRepo{db: d} }

func (r *RecommendationRepo) UpsertBatch(ctx context.Context, recs []models.Recommendation) error {
	if len(recs) == 0 {
		return nil
	}
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	batch := &pgx.Batch{}
	for _, r := range recs {
		batch.Queue(`
            INSERT INTO recommendations (user_id, promo_id, score, reason, generated_at)
            VALUES ($1, $2, $3, $4, $5)
            ON CONFLICT (user_id, promo_id) DO UPDATE
            SET score = EXCLUDED.score, reason = EXCLUDED.reason, generated_at = EXCLUDED.generated_at
        `, r.UserID, r.PromoID, r.Score, r.Reason, r.GeneratedAt)
	}
	br := tx.SendBatch(ctx, batch)
	for range recs {
		if _, err := br.Exec(); err != nil {
			br.Close()
			return fmt.Errorf("upsert rec: %w", err)
		}
	}
	if err := br.Close(); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *RecommendationRepo) ForUser(ctx context.Context, userID int64, limit int) ([]models.Recommendation, error) {
	if limit <= 0 || limit > 200 {
		limit = 30
	}
	const q = `
        SELECT user_id, promo_id, score, reason, generated_at
        FROM recommendations
        WHERE user_id = $1
        ORDER BY score DESC LIMIT $2
    `
	rows, err := r.db.Pool.Query(ctx, q, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.Recommendation, 0, limit)
	for rows.Next() {
		var rec models.Recommendation
		if err := rows.Scan(&rec.UserID, &rec.PromoID, &rec.Score, &rec.Reason, &rec.GeneratedAt); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (r *RecommendationRepo) DeleteOlderThan(ctx context.Context, age time.Duration) (int64, error) {
	cutoff := time.Now().UTC().Add(-age)
	ct, err := r.db.Pool.Exec(ctx, `DELETE FROM recommendations WHERE generated_at < $1`, cutoff)
	if err != nil {
		return 0, err
	}
	return ct.RowsAffected(), nil
}
