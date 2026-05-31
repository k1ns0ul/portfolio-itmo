package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/andrey/t-vygoda/internal/db"
	"github.com/andrey/t-vygoda/internal/models"
)

type CFARepo struct {
	db *db.DB
}

func NewCFARepo(d *db.DB) *CFARepo { return &CFARepo{db: d} }

type CreateSettlementInput struct {
	PurchaseID int64
	PartnerID  int64
	DebtorType models.DebtorType
	Amount     float64
}

func (r *CFARepo) CreateSettlement(ctx context.Context, in CreateSettlementInput) (*models.CFASettlement, error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	const insertQ = `
        INSERT INTO cfa_settlements (purchase_id, partner_id, debtor_type, amount, status)
        VALUES ($1, $2, $3, $4, 'created')
        RETURNING id, purchase_id, partner_id, debtor_type, amount, status, created_at, settled_at
    `
	var s models.CFASettlement
	err = tx.QueryRow(ctx, insertQ, in.PurchaseID, in.PartnerID, in.DebtorType, in.Amount).Scan(
		&s.ID, &s.PurchaseID, &s.PartnerID, &s.DebtorType, &s.Amount, &s.Status, &s.CreatedAt, &s.SettledAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrDuplicate
		}
		return nil, fmt.Errorf("insert settlement: %w", err)
	}

	if err := upsertBalance(ctx, tx, in.PartnerID, in.DebtorType, in.Amount); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return &s, nil
}

func upsertBalance(ctx context.Context, tx pgx.Tx, partnerID int64, debtor models.DebtorType, amount float64) error {
	const q = `
        INSERT INTO cfa_balances (partner_id, bank_owes, partner_owes, updated_at)
        VALUES ($1, $2, $3, now())
        ON CONFLICT (partner_id) DO UPDATE
        SET bank_owes    = cfa_balances.bank_owes + EXCLUDED.bank_owes,
            partner_owes = cfa_balances.partner_owes + EXCLUDED.partner_owes,
            updated_at   = now()
    `
	bankOwes, partnerOwes := 0.0, 0.0
	if debtor == models.DebtorBank {
		bankOwes = amount
	} else {
		partnerOwes = amount
	}
	if _, err := tx.Exec(ctx, q, partnerID, bankOwes, partnerOwes); err != nil {
		return fmt.Errorf("upsert balance: %w", err)
	}
	return nil
}

func (r *CFARepo) GetSettlement(ctx context.Context, id int64) (*models.CFASettlement, error) {
	const q = `
        SELECT id, purchase_id, partner_id, debtor_type, amount, status, created_at, settled_at
        FROM cfa_settlements WHERE id = $1
    `
	var s models.CFASettlement
	err := r.db.Pool.QueryRow(ctx, q, id).Scan(
		&s.ID, &s.PurchaseID, &s.PartnerID, &s.DebtorType, &s.Amount, &s.Status, &s.CreatedAt, &s.SettledAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get settlement: %w", err)
	}
	return &s, nil
}

func (r *CFARepo) ListByPartner(ctx context.Context, partnerID int64, from, to time.Time, status models.CFAStatus, limit int) ([]models.CFASettlement, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	args := []any{partnerID, from, to}
	q := `
        SELECT id, purchase_id, partner_id, debtor_type, amount, status, created_at, settled_at
        FROM cfa_settlements
        WHERE partner_id = $1 AND created_at >= $2 AND created_at < $3
    `
	if status != "" {
		args = append(args, status)
		q += fmt.Sprintf(" AND status = $%d", len(args))
	}
	args = append(args, limit)
	q += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", len(args))

	rows, err := r.db.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list settlements: %w", err)
	}
	defer rows.Close()
	out := make([]models.CFASettlement, 0, 16)
	for rows.Next() {
		var s models.CFASettlement
		if err := rows.Scan(&s.ID, &s.PurchaseID, &s.PartnerID, &s.DebtorType, &s.Amount, &s.Status, &s.CreatedAt, &s.SettledAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *CFARepo) GetBalance(ctx context.Context, partnerID int64) (*models.CFABalance, error) {
	const q = `
        SELECT partner_id, bank_owes, partner_owes, net_balance, updated_at
        FROM cfa_balances WHERE partner_id = $1
    `
	var b models.CFABalance
	err := r.db.Pool.QueryRow(ctx, q, partnerID).Scan(&b.PartnerID, &b.BankOwes, &b.PartnerOwes, &b.NetBalance, &b.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get balance: %w", err)
	}
	return &b, nil
}

func (r *CFARepo) ListBalances(ctx context.Context) ([]models.CFABalance, error) {
	rows, err := r.db.Pool.Query(ctx, `
        SELECT partner_id, bank_owes, partner_owes, net_balance, updated_at
        FROM cfa_balances ORDER BY ABS(net_balance) DESC
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.CFABalance, 0, 32)
	for rows.Next() {
		var b models.CFABalance
		if err := rows.Scan(&b.PartnerID, &b.BankOwes, &b.PartnerOwes, &b.NetBalance, &b.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (r *CFARepo) Reconcile(ctx context.Context, partnerID int64) (*models.CFAReconciliation, error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var net float64
	err = tx.QueryRow(ctx, `
        SELECT net_balance FROM cfa_balances WHERE partner_id = $1 FOR UPDATE
    `, partnerID).Scan(&net)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock balance: %w", err)
	}

	if _, err := tx.Exec(ctx, `
        UPDATE cfa_settlements
        SET status = 'settled', settled_at = now()
        WHERE partner_id = $1 AND status IN ('created', 'confirmed')
    `, partnerID); err != nil {
		return nil, fmt.Errorf("mark settled: %w", err)
	}

	if _, err := tx.Exec(ctx, `
        UPDATE cfa_balances SET bank_owes = 0, partner_owes = 0, updated_at = now()
        WHERE partner_id = $1
    `, partnerID); err != nil {
		return nil, fmt.Errorf("reset balance: %w", err)
	}

	var rec models.CFAReconciliation
	err = tx.QueryRow(ctx, `
        INSERT INTO cfa_reconciliations (partner_id, settled_amount)
        VALUES ($1, $2)
        RETURNING id, partner_id, settled_amount, settled_at
    `, partnerID, net).Scan(&rec.ID, &rec.PartnerID, &rec.SettledAmount, &rec.SettledAt)
	if err != nil {
		return nil, fmt.Errorf("create reconciliation: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return &rec, nil
}

func (r *CFARepo) ConfirmSettlement(ctx context.Context, id int64) error {
	ct, err := r.db.Pool.Exec(ctx, `
        UPDATE cfa_settlements SET status = 'confirmed'
        WHERE id = $1 AND status = 'created'
    `, id)
	if err != nil {
		return fmt.Errorf("confirm settlement: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
