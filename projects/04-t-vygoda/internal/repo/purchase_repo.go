package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/andrey/t-vygoda/internal/db"
	"github.com/andrey/t-vygoda/internal/models"
)

type PurchaseRepo struct {
	db *db.DB
}

func NewPurchaseRepo(d *db.DB) *PurchaseRepo { return &PurchaseRepo{db: d} }

func (r *PurchaseRepo) Create(ctx context.Context, userID, promoID, partnerID int64, amount float64) (*models.Purchase, error) {
	const q = `
        INSERT INTO purchases (user_id, promo_id, partner_id, amount, status)
        VALUES ($1, $2, $3, $4, 'pending')
        RETURNING id, user_id, promo_id, partner_id, amount, cpa_amount, status, created_at, confirmed_at
    `
	var p models.Purchase
	err := r.db.Pool.QueryRow(ctx, q, userID, promoID, partnerID, amount).Scan(
		&p.ID, &p.UserID, &p.PromoID, &p.PartnerID, &p.Amount, &p.CPAAmount, &p.Status, &p.CreatedAt, &p.ConfirmedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert purchase: %w", err)
	}
	return &p, nil
}

func (r *PurchaseRepo) GetByID(ctx context.Context, id int64) (*models.Purchase, error) {
	const q = `
        SELECT id, user_id, promo_id, partner_id, amount, cpa_amount, status, created_at, confirmed_at
        FROM purchases WHERE id = $1
    `
	var p models.Purchase
	err := r.db.Pool.QueryRow(ctx, q, id).Scan(
		&p.ID, &p.UserID, &p.PromoID, &p.PartnerID, &p.Amount, &p.CPAAmount, &p.Status, &p.CreatedAt, &p.ConfirmedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get purchase: %w", err)
	}
	return &p, nil
}

func (r *PurchaseRepo) Confirm(ctx context.Context, id int64, cpaAmount float64) (*models.Purchase, error) {
	const q = `
        UPDATE purchases
        SET status = 'confirmed', cpa_amount = $2, confirmed_at = now()
        WHERE id = $1 AND status = 'pending'
        RETURNING id, user_id, promo_id, partner_id, amount, cpa_amount, status, created_at, confirmed_at
    `
	var p models.Purchase
	err := r.db.Pool.QueryRow(ctx, q, id, cpaAmount).Scan(
		&p.ID, &p.UserID, &p.PromoID, &p.PartnerID, &p.Amount, &p.CPAAmount, &p.Status, &p.CreatedAt, &p.ConfirmedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("confirm purchase: %w", err)
	}
	return &p, nil
}

func (r *PurchaseRepo) Cancel(ctx context.Context, id int64) error {
	ct, err := r.db.Pool.Exec(ctx, `
        UPDATE purchases SET status = 'cancelled' WHERE id = $1 AND status = 'pending'
    `, id)
	if err != nil {
		return fmt.Errorf("cancel purchase: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PurchaseRepo) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]models.Purchase, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	const q = `
        SELECT id, user_id, promo_id, partner_id, amount, cpa_amount, status, created_at, confirmed_at
        FROM purchases WHERE user_id = $1
        ORDER BY created_at DESC LIMIT $2 OFFSET $3
    `
	rows, err := r.db.Pool.Query(ctx, q, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPurchases(rows)
}

func (r *PurchaseRepo) ListByPartner(ctx context.Context, partnerID int64, from, to time.Time, limit int) ([]models.Purchase, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	const q = `
        SELECT id, user_id, promo_id, partner_id, amount, cpa_amount, status, created_at, confirmed_at
        FROM purchases
        WHERE partner_id = $1 AND created_at >= $2 AND created_at < $3
        ORDER BY created_at DESC LIMIT $4
    `
	rows, err := r.db.Pool.Query(ctx, q, partnerID, from, to, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPurchases(rows)
}

type ConfirmedSummary struct {
	Count    uint64
	Total    float64
	AvgCheck float64
}

func (r *PurchaseRepo) ConfirmedSummary(ctx context.Context, from, to time.Time) (ConfirmedSummary, error) {
	const q = `
        SELECT count(*), COALESCE(SUM(amount), 0), COALESCE(AVG(amount), 0)
        FROM purchases
        WHERE status = 'confirmed' AND created_at >= $1 AND created_at < $2
    `
	var s ConfirmedSummary
	if err := r.db.Pool.QueryRow(ctx, q, from, to).Scan(&s.Count, &s.Total, &s.AvgCheck); err != nil {
		return s, fmt.Errorf("confirmed summary: %w", err)
	}
	return s, nil
}

func (r *PurchaseRepo) SumByPartner(ctx context.Context, partnerID int64, from, to time.Time) (float64, float64, error) {
	const q = `
        SELECT COALESCE(SUM(amount), 0), COALESCE(SUM(cpa_amount), 0)
        FROM purchases
        WHERE partner_id = $1 AND status = 'confirmed'
          AND created_at >= $2 AND created_at < $3
    `
	var total, cpa float64
	if err := r.db.Pool.QueryRow(ctx, q, partnerID, from, to).Scan(&total, &cpa); err != nil {
		return 0, 0, fmt.Errorf("sum partner: %w", err)
	}
	return total, cpa, nil
}

func scanPurchases(rows pgx.Rows) ([]models.Purchase, error) {
	out := make([]models.Purchase, 0, 16)
	for rows.Next() {
		var p models.Purchase
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.PromoID, &p.PartnerID, &p.Amount,
			&p.CPAAmount, &p.Status, &p.CreatedAt, &p.ConfirmedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
