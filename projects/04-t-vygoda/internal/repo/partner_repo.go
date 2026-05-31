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

type PartnerRepo struct {
	db *db.DB
}

func NewPartnerRepo(d *db.DB) *PartnerRepo { return &PartnerRepo{db: d} }

func (r *PartnerRepo) Create(ctx context.Context, in models.PartnerInput) (*models.Partner, error) {
	const q = `
        INSERT INTO partners (name, logo_url, cpa_percent, contact_email, active)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING id, name, logo_url, cpa_percent, contact_email, active, created_at, updated_at
    `
	var p models.Partner
	err := r.db.Pool.QueryRow(ctx, q,
		in.Name, in.LogoURL, in.CPAPercent, in.ContactEmail, in.Active,
	).Scan(&p.ID, &p.Name, &p.LogoURL, &p.CPAPercent, &p.ContactEmail, &p.Active, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert partner: %w", err)
	}
	return &p, nil
}

func (r *PartnerRepo) Update(ctx context.Context, id int64, in models.PartnerInput) (*models.Partner, error) {
	const q = `
        UPDATE partners
        SET name = $2, logo_url = $3, cpa_percent = $4, contact_email = $5, active = $6, updated_at = now()
        WHERE id = $1
        RETURNING id, name, logo_url, cpa_percent, contact_email, active, created_at, updated_at
    `
	var p models.Partner
	err := r.db.Pool.QueryRow(ctx, q, id, in.Name, in.LogoURL, in.CPAPercent, in.ContactEmail, in.Active).Scan(
		&p.ID, &p.Name, &p.LogoURL, &p.CPAPercent, &p.ContactEmail, &p.Active, &p.CreatedAt, &p.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update partner: %w", err)
	}
	return &p, nil
}

func (r *PartnerRepo) GetByID(ctx context.Context, id int64) (*models.Partner, error) {
	const q = `
        SELECT id, name, logo_url, cpa_percent, contact_email, active, created_at, updated_at
        FROM partners WHERE id = $1
    `
	var p models.Partner
	err := r.db.Pool.QueryRow(ctx, q, id).Scan(
		&p.ID, &p.Name, &p.LogoURL, &p.CPAPercent, &p.ContactEmail, &p.Active, &p.CreatedAt, &p.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get partner: %w", err)
	}
	return &p, nil
}

func (r *PartnerRepo) ListActive(ctx context.Context, limit int) ([]models.Partner, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.db.Pool.Query(ctx, `
        SELECT id, name, logo_url, cpa_percent, contact_email, active, created_at, updated_at
        FROM partners WHERE active = TRUE ORDER BY name ASC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.Partner, 0, 32)
	for rows.Next() {
		var p models.Partner
		if err := rows.Scan(&p.ID, &p.Name, &p.LogoURL, &p.CPAPercent, &p.ContactEmail, &p.Active, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *PartnerRepo) WithBalance(ctx context.Context, id int64) (*models.PartnerWithBalance, error) {
	const q = `
        SELECT p.id, p.name, p.logo_url, p.cpa_percent, p.contact_email, p.active, p.created_at, p.updated_at,
               b.partner_id, b.bank_owes, b.partner_owes, b.net_balance, b.updated_at
        FROM partners p
        LEFT JOIN cfa_balances b ON b.partner_id = p.id
        WHERE p.id = $1
    `
	var out models.PartnerWithBalance
	var bPartner *int64
	var bBank, bPartnerOwes, bNet *float64
	var bUpdated *time.Time
	err := r.db.Pool.QueryRow(ctx, q, id).Scan(
		&out.ID, &out.Name, &out.LogoURL, &out.CPAPercent, &out.ContactEmail, &out.Active, &out.CreatedAt, &out.UpdatedAt,
		&bPartner, &bBank, &bPartnerOwes, &bNet, &bUpdated,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("partner with balance: %w", err)
	}
	if bPartner != nil {
		bal := &models.CFABalance{
			PartnerID:   *bPartner,
			BankOwes:    deref(bBank),
			PartnerOwes: deref(bPartnerOwes),
			NetBalance:  deref(bNet),
		}
		if bUpdated != nil {
			bal.UpdatedAt = *bUpdated
		}
		out.Balance = bal
	}
	return &out, nil
}

func deref(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}
