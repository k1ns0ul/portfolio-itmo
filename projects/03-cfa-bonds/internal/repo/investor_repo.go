package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/andrey/cfa-bonds/internal/db"
	"github.com/andrey/cfa-bonds/internal/models"
)

type InvestorRepo struct {
	pool *pgxpool.Pool
}

func NewInvestorRepo(pool *pgxpool.Pool) *InvestorRepo {
	return &InvestorRepo{pool: pool}
}

func (r *InvestorRepo) Create(ctx context.Context, inv *models.Investor) error {
	if inv.ID == uuid.Nil {
		inv.ID = uuid.New()
	}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO investors (id, name, type, account_number, balance)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING created_at`,
		inv.ID, inv.Name, inv.Type, inv.AccountNumber, db.NumericFromDecimal(inv.Balance)).Scan(&inv.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert investor %s: %w", inv.AccountNumber, err)
	}
	return nil
}

func (r *InvestorRepo) Get(ctx context.Context, id uuid.UUID) (*models.Investor, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, name, type, account_number, balance, created_at
		FROM investors WHERE id=$1`, id)
	inv, err := scanInvestor(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get investor %s: %w", id, err)
	}
	return inv, nil
}

func (r *InvestorRepo) GetByAccountNumber(ctx context.Context, acc string) (*models.Investor, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, name, type, account_number, balance, created_at
		FROM investors WHERE account_number=$1`, acc)
	inv, err := scanInvestor(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lookup investor by account %s: %w", acc, err)
	}
	return inv, nil
}

func (r *InvestorRepo) UpdateBalance(ctx context.Context, id uuid.UUID, delta decimal.Decimal) error {
	return r.UpdateBalanceTx(ctx, r.pool, id, delta)
}

func (r *InvestorRepo) UpdateBalanceTx(ctx context.Context, q Queryer, id uuid.UUID, delta decimal.Decimal) error {
	tag, err := q.Exec(ctx, `
		UPDATE investors SET balance = balance + $1 WHERE id = $2`,
		db.NumericFromDecimal(delta), id)
	if err != nil {
		return fmt.Errorf("update balance of %s by %s: %w", id, delta.String(), err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("investor %s missing during balance update: %w", id, ErrNotFound)
	}
	return nil
}

func (r *InvestorRepo) CountAll(ctx context.Context) (int64, error) {
	var n int64
	if err := r.pool.QueryRow(ctx, "SELECT count(*) FROM investors").Scan(&n); err != nil {
		return 0, fmt.Errorf("count investors: %w", err)
	}
	return n, nil
}

func (r *InvestorRepo) ListWithPositions(ctx context.Context, limit, offset int) ([]*models.Investor, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT i.id, i.name, i.type, i.account_number, i.balance, i.created_at
		FROM investors i
		JOIN positions p ON p.investor_id = i.id AND p.quantity > 0
		ORDER BY i.created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list investors with positions: %w", err)
	}
	defer rows.Close()
	var out []*models.Investor
	for rows.Next() {
		inv, err := scanInvestor(rows)
		if err != nil {
			return nil, fmt.Errorf("scan investor: %w", err)
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

func scanInvestor(row pgx.Row) (*models.Investor, error) {
	var inv models.Investor
	var bal pgxNumeric
	err := row.Scan(&inv.ID, &inv.Name, &inv.Type, &inv.AccountNumber, &bal, &inv.CreatedAt)
	if err != nil {
		return nil, err
	}
	inv.Balance, err = db.DecimalFromNumeric(bal)
	if err != nil {
		return nil, fmt.Errorf("decode balance: %w", err)
	}
	return &inv, nil
}
