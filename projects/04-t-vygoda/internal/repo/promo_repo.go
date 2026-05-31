package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/andrey/t-vygoda/internal/db"
	"github.com/andrey/t-vygoda/internal/models"
)

type PromoRepo struct {
	db *db.DB
}

func NewPromoRepo(d *db.DB) *PromoRepo { return &PromoRepo{db: d} }

func (r *PromoRepo) Create(ctx context.Context, in models.CreatePromoInput) (*models.Promo, error) {
	const q = `
        INSERT INTO promos (partner_id, code, discount, type, category_id, max_uses, expires_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        RETURNING id, partner_id, code, discount, type, category_id, max_uses, current_uses, expires_at, active, created_at
    `
	var p models.Promo
	err := r.db.Pool.QueryRow(ctx, q,
		in.PartnerID, in.Code, in.Discount, in.Type, in.CategoryID, in.MaxUses, in.ExpiresAt,
	).Scan(
		&p.ID, &p.PartnerID, &p.Code, &p.Discount, &p.Type, &p.CategoryID,
		&p.MaxUses, &p.CurrentUses, &p.ExpiresAt, &p.Active, &p.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrDuplicate
		}
		return nil, fmt.Errorf("insert promo: %w", err)
	}
	return &p, nil
}

func (r *PromoRepo) GetByID(ctx context.Context, id int64) (*models.Promo, error) {
	return r.scanOne(ctx, "WHERE id = $1", id)
}

func (r *PromoRepo) GetByCode(ctx context.Context, code string) (*models.Promo, error) {
	return r.scanOne(ctx, "WHERE code = $1", code)
}

func (r *PromoRepo) ListByPartner(ctx context.Context, partnerID int64) ([]models.Promo, error) {
	rows, err := r.db.Pool.Query(ctx, baseSelect+" WHERE partner_id = $1 ORDER BY created_at DESC", partnerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPromos(rows)
}

func (r *PromoRepo) ListActive(ctx context.Context, categoryID *int64, limit int) ([]models.Promo, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if categoryID != nil {
		rows, err := r.db.Pool.Query(ctx, baseSelect+`
            WHERE active = TRUE
              AND (expires_at IS NULL OR expires_at > now())
              AND (max_uses = 0 OR current_uses < max_uses)
              AND category_id = $1
            ORDER BY created_at DESC LIMIT $2`, *categoryID, limit)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		return scanPromos(rows)
	}
	rows, err := r.db.Pool.Query(ctx, baseSelect+`
        WHERE active = TRUE
          AND (expires_at IS NULL OR expires_at > now())
          AND (max_uses = 0 OR current_uses < max_uses)
        ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPromos(rows)
}

func (r *PromoRepo) IncrementUses(ctx context.Context, id int64) error {
	ct, err := r.db.Pool.Exec(ctx, `
        UPDATE promos SET current_uses = current_uses + 1
        WHERE id = $1
          AND active = TRUE
          AND (expires_at IS NULL OR expires_at > now())
          AND (max_uses = 0 OR current_uses < max_uses)
    `, id)
	if err != nil {
		return fmt.Errorf("incr promo: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrUnavailable
	}
	return nil
}

func (r *PromoRepo) Popular(ctx context.Context, limit int) ([]models.Promo, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := r.db.Pool.Query(ctx, baseSelect+`
        WHERE active = TRUE
          AND (expires_at IS NULL OR expires_at > now())
        ORDER BY current_uses DESC, created_at DESC
        LIMIT $1
    `, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPromos(rows)
}

const baseSelect = `
    SELECT id, partner_id, code, discount, type, category_id, max_uses, current_uses, expires_at, active, created_at
    FROM promos
`

func (r *PromoRepo) scanOne(ctx context.Context, where string, args ...any) (*models.Promo, error) {
	var p models.Promo
	err := r.db.Pool.QueryRow(ctx, baseSelect+where+" LIMIT 1", args...).Scan(
		&p.ID, &p.PartnerID, &p.Code, &p.Discount, &p.Type, &p.CategoryID,
		&p.MaxUses, &p.CurrentUses, &p.ExpiresAt, &p.Active, &p.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get promo: %w", err)
	}
	return &p, nil
}

func scanPromos(rows pgx.Rows) ([]models.Promo, error) {
	out := make([]models.Promo, 0, 16)
	for rows.Next() {
		var p models.Promo
		if err := rows.Scan(
			&p.ID, &p.PartnerID, &p.Code, &p.Discount, &p.Type, &p.CategoryID,
			&p.MaxUses, &p.CurrentUses, &p.ExpiresAt, &p.Active, &p.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
