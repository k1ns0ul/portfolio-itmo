package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/andrey/cfa-bonds/internal/models"
)

var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("state conflict")

type IssuerRepo struct {
	pool *pgxpool.Pool
}

func NewIssuerRepo(pool *pgxpool.Pool) *IssuerRepo {
	return &IssuerRepo{pool: pool}
}

func (r *IssuerRepo) Create(ctx context.Context, is *models.Issuer) error {
	if is.ID == uuid.Nil {
		is.ID = uuid.New()
	}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO issuers (id, name, inn, ogrn, contact_email, active)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING created_at`,
		is.ID, is.Name, is.INN, is.OGRN, is.ContactEmail, is.Active).Scan(&is.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert issuer %s: %w", is.INN, err)
	}
	return nil
}

func (r *IssuerRepo) Get(ctx context.Context, id uuid.UUID) (*models.Issuer, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, name, inn, ogrn, contact_email, active, created_at
		FROM issuers WHERE id=$1`, id)
	is, err := scanIssuer(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get issuer %s: %w", id, err)
	}
	return is, nil
}

func (r *IssuerRepo) GetByINN(ctx context.Context, inn string) (*models.Issuer, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, name, inn, ogrn, contact_email, active, created_at
		FROM issuers WHERE inn=$1`, inn)
	is, err := scanIssuer(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lookup issuer by inn %s: %w", inn, err)
	}
	return is, nil
}

func (r *IssuerRepo) List(ctx context.Context, limit, offset int) ([]*models.Issuer, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, "SELECT count(*) FROM issuers").Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count issuers: %w", err)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, inn, ogrn, contact_email, active, created_at
		FROM issuers ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query issuers: %w", err)
	}
	defer rows.Close()
	out, err := collectIssuers(rows)
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (r *IssuerRepo) ListActive(ctx context.Context) ([]*models.Issuer, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, inn, ogrn, contact_email, active, created_at
		FROM issuers WHERE active = true ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query active issuers: %w", err)
	}
	defer rows.Close()
	return collectIssuers(rows)
}

func scanIssuer(row pgx.Row) (*models.Issuer, error) {
	var is models.Issuer
	err := row.Scan(&is.ID, &is.Name, &is.INN, &is.OGRN, &is.ContactEmail, &is.Active, &is.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &is, nil
}

func collectIssuers(rows pgx.Rows) ([]*models.Issuer, error) {
	var out []*models.Issuer
	for rows.Next() {
		is, err := scanIssuer(rows)
		if err != nil {
			return nil, fmt.Errorf("scan issuer: %w", err)
		}
		out = append(out, is)
	}
	return out, rows.Err()
}
