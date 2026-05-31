package repo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/andrey/cfa-bonds/internal/db"
	"github.com/andrey/cfa-bonds/internal/models"
)

type IssueRepo struct {
	pool *pgxpool.Pool
}

func NewIssueRepo(pool *pgxpool.Pool) *IssueRepo {
	return &IssueRepo{pool: pool}
}

type IssueFilter struct {
	Status   string
	IssuerID uuid.UUID
	Limit    int
	Offset   int
}

const issueColumns = `id, issuer_id, name, isin, nominal, coupon_rate, coupon_frequency,
	issue_date, maturity_date, total_quantity, placed_quantity, status, created_at, updated_at`

func (r *IssueRepo) Create(ctx context.Context, bi *models.BondIssue) error {
	if bi.ID == uuid.Nil {
		bi.ID = uuid.New()
	}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO bond_issues (id, issuer_id, name, isin, nominal, coupon_rate, coupon_frequency,
			issue_date, maturity_date, total_quantity, placed_quantity, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING created_at, updated_at`,
		bi.ID, bi.IssuerID, bi.Name, bi.ISIN, db.NumericFromDecimal(bi.Nominal),
		db.NumericFromDecimal(bi.CouponRate), bi.CouponFrequency, bi.IssueDate, bi.MaturityDate,
		bi.TotalQuantity, bi.PlacedQuantity, bi.Status).Scan(&bi.CreatedAt, &bi.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert issue %s: %w", bi.ISIN, err)
	}
	return nil
}

func (r *IssueRepo) Get(ctx context.Context, id uuid.UUID) (*models.BondIssue, error) {
	row := r.pool.QueryRow(ctx, "SELECT "+issueColumns+" FROM bond_issues WHERE id=$1", id)
	bi, err := scanIssue(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get issue %s: %w", id, err)
	}
	return bi, nil
}

func (r *IssueRepo) List(ctx context.Context, f IssueFilter) ([]*models.BondIssue, int, error) {
	var conds []string
	var args []any
	if f.Status != "" {
		args = append(args, f.Status)
		conds = append(conds, fmt.Sprintf("status = $%d", len(args)))
	}
	if f.IssuerID != uuid.Nil {
		args = append(args, f.IssuerID)
		conds = append(conds, fmt.Sprintf("issuer_id = $%d", len(args)))
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT count(*) FROM bond_issues "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count issues: %w", err)
	}

	limit := clampLimit(f.Limit)
	offset := clampOffset(f.Offset)
	args = append(args, limit, offset)
	query := fmt.Sprintf("SELECT %s FROM bond_issues %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d",
		issueColumns, where, len(args)-1, len(args))

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query issues: %w", err)
	}
	defer rows.Close()
	out, err := collectIssues(rows)
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (r *IssueRepo) UpdateStatus(ctx context.Context, id uuid.UUID, expected, next string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE bond_issues SET status=$1, updated_at=now() WHERE id=$2 AND status=$3`,
		next, id, expected)
	if err != nil {
		return fmt.Errorf("transition issue %s %s->%s: %w", id, expected, next, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("issue %s not in expected state %s: %w", id, expected, ErrConflict)
	}
	return nil
}

func (r *IssueRepo) IncrementPlaced(ctx context.Context, q Queryer, id uuid.UUID, delta int64) (int64, error) {
	var placed, total int64
	err := q.QueryRow(ctx, `
		UPDATE bond_issues
		SET placed_quantity = placed_quantity + $1, updated_at = now()
		WHERE id = $2 AND placed_quantity + $1 <= total_quantity
		RETURNING placed_quantity, total_quantity`, delta, id).Scan(&placed, &total)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, fmt.Errorf("placement of %d exceeds remaining for issue %s: %w", delta, id, ErrConflict)
	}
	if err != nil {
		return 0, fmt.Errorf("increment placed for %s: %w", id, err)
	}
	return placed, nil
}

func (r *IssueRepo) GetActiveWithCouponsDue(ctx context.Context, asOf time.Time) ([]*models.BondIssue, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+issueColumns+`
		FROM bond_issues bi
		WHERE bi.status = $1
		  AND EXISTS (
			SELECT 1 FROM coupon_schedule cs
			WHERE cs.issue_id = bi.id AND cs.status = $2 AND cs.payment_date <= $3
		  )
		ORDER BY bi.maturity_date`, models.IssueActive, models.CouponScheduled, asOf)
	if err != nil {
		return nil, fmt.Errorf("query issues with due coupons: %w", err)
	}
	defer rows.Close()
	return collectIssues(rows)
}

func (r *IssueRepo) GetMaturingIssues(ctx context.Context, asOf time.Time) ([]*models.BondIssue, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+issueColumns+`
		FROM bond_issues
		WHERE status = $1 AND maturity_date <= $2
		ORDER BY maturity_date`, models.IssueActive, asOf)
	if err != nil {
		return nil, fmt.Errorf("query maturing issues: %w", err)
	}
	defer rows.Close()
	return collectIssues(rows)
}

func (r *IssueRepo) CountByStatus(ctx context.Context) (map[string]int64, error) {
	rows, err := r.pool.Query(ctx, "SELECT status, count(*) FROM bond_issues GROUP BY status")
	if err != nil {
		return nil, fmt.Errorf("count issues by status: %w", err)
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var s string
		var c int64
		if err := rows.Scan(&s, &c); err != nil {
			return nil, fmt.Errorf("scan status count: %w", err)
		}
		out[s] = c
	}
	return out, rows.Err()
}

func scanIssue(row pgx.Row) (*models.BondIssue, error) {
	var bi models.BondIssue
	var nominal, rate pgxNumeric
	err := row.Scan(&bi.ID, &bi.IssuerID, &bi.Name, &bi.ISIN, &nominal, &rate, &bi.CouponFrequency,
		&bi.IssueDate, &bi.MaturityDate, &bi.TotalQuantity, &bi.PlacedQuantity, &bi.Status,
		&bi.CreatedAt, &bi.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if bi.Nominal, err = db.DecimalFromNumeric(nominal); err != nil {
		return nil, fmt.Errorf("decode nominal: %w", err)
	}
	if bi.CouponRate, err = db.DecimalFromNumeric(rate); err != nil {
		return nil, fmt.Errorf("decode coupon rate: %w", err)
	}
	return &bi, nil
}

func collectIssues(rows pgx.Rows) ([]*models.BondIssue, error) {
	var out []*models.BondIssue
	for rows.Next() {
		bi, err := scanIssue(rows)
		if err != nil {
			return nil, fmt.Errorf("scan issue: %w", err)
		}
		out = append(out, bi)
	}
	return out, rows.Err()
}
