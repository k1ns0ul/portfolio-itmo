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

type ReferralRepo struct {
	db *db.DB
}

func NewReferralRepo(d *db.DB) *ReferralRepo { return &ReferralRepo{db: d} }

func (r *ReferralRepo) BuildChainFor(ctx context.Context, newUserID, directReferrerID int64) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	const buildQ = `
        WITH RECURSIVE ancestors AS (
            SELECT id, referred_by, 1 AS level FROM users WHERE id = $1
            UNION ALL
            SELECT u.id, u.referred_by, a.level + 1
            FROM users u
            JOIN ancestors a ON a.referred_by = u.id
            WHERE a.level < 3
        )
        INSERT INTO referral_chains (user_id, referrer_id, level)
        SELECT $2, id, level FROM ancestors
        ON CONFLICT (user_id, referrer_id, level) DO NOTHING
    `
	if _, err := tx.Exec(ctx, buildQ, directReferrerID, newUserID); err != nil {
		return fmt.Errorf("build chain: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE users SET referred_by = $1 WHERE id = $2 AND referred_by IS NULL`,
		directReferrerID, newUserID); err != nil {
		return fmt.Errorf("set referred_by: %w", err)
	}
	return tx.Commit(ctx)
}

func (r *ReferralRepo) GetChainsForUser(ctx context.Context, userID int64) ([]models.ReferralTreeNode, error) {
	const q = `
        SELECT id, referrer_id, level
        FROM referral_chains WHERE user_id = $1
        ORDER BY level ASC
    `
	rows, err := r.db.Pool.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.ReferralTreeNode, 0, 3)
	for rows.Next() {
		var n models.ReferralTreeNode
		if err := rows.Scan(&n.ChainID, &n.ReferrerID, &n.Level); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (r *ReferralRepo) ListReferralsByReferrer(ctx context.Context, referrerID int64, level *models.ReferralLevel) ([]models.ReferralChain, error) {
	if level != nil {
		rows, err := r.db.Pool.Query(ctx, `
            SELECT id, user_id, referrer_id, level, created_at
            FROM referral_chains WHERE referrer_id = $1 AND level = $2
            ORDER BY created_at DESC`, referrerID, int(*level))
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		return scanChains(rows)
	}
	rows, err := r.db.Pool.Query(ctx, `
        SELECT id, user_id, referrer_id, level, created_at
        FROM referral_chains WHERE referrer_id = $1
        ORDER BY level ASC, created_at DESC`, referrerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanChains(rows)
}

func (r *ReferralRepo) GetTreeUpTo3Levels(ctx context.Context, userID int64) ([]models.ReferralTreeNode, error) {
	const q = `
        WITH RECURSIVE tree AS (
            SELECT c.id AS chain_id, c.user_id, c.referrer_id, c.level
            FROM referral_chains c
            WHERE c.user_id = $1
            UNION ALL
            SELECT c.id, c.user_id, c.referrer_id, c.level
            FROM referral_chains c
            JOIN tree t ON c.user_id = t.referrer_id
            WHERE c.level < 3
        )
        SELECT chain_id, referrer_id, level FROM tree
        ORDER BY level ASC
    `
	rows, err := r.db.Pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("ref tree: %w", err)
	}
	defer rows.Close()
	seen := map[int64]struct{}{}
	out := make([]models.ReferralTreeNode, 0, 3)
	for rows.Next() {
		var n models.ReferralTreeNode
		if err := rows.Scan(&n.ChainID, &n.ReferrerID, &n.Level); err != nil {
			return nil, err
		}
		if _, ok := seen[n.ReferrerID]; ok {
			continue
		}
		seen[n.ReferrerID] = struct{}{}
		out = append(out, n)
	}
	return out, rows.Err()
}

type CreateBonusInput struct {
	ChainID    int64
	PurchaseID int64
	ReferrerID int64
	Amount     float64
	Level      models.ReferralLevel
}

func (r *ReferralRepo) CreateBonus(ctx context.Context, in CreateBonusInput) (*models.ReferralBonus, error) {
	const q = `
        INSERT INTO referral_bonuses (chain_id, purchase_id, referrer_id, amount, level)
        VALUES ($1, $2, $3, $4, $5)
        ON CONFLICT (purchase_id, referrer_id, level) DO NOTHING
        RETURNING id, chain_id, purchase_id, referrer_id, amount, level, status, created_at
    `
	var b models.ReferralBonus
	err := r.db.Pool.QueryRow(ctx, q, in.ChainID, in.PurchaseID, in.ReferrerID, in.Amount, int(in.Level)).Scan(
		&b.ID, &b.ChainID, &b.PurchaseID, &b.ReferrerID, &b.Amount, &b.Level, &b.Status, &b.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrDuplicate
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrDuplicate
		}
		return nil, fmt.Errorf("create bonus: %w", err)
	}
	return &b, nil
}

func (r *ReferralRepo) ListBonusesByUser(ctx context.Context, referrerID int64, limit int) ([]models.ReferralBonus, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	const q = `
        SELECT id, chain_id, purchase_id, referrer_id, amount, level, status, created_at
        FROM referral_bonuses
        WHERE referrer_id = $1
        ORDER BY created_at DESC LIMIT $2
    `
	rows, err := r.db.Pool.Query(ctx, q, referrerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.ReferralBonus, 0, 16)
	for rows.Next() {
		var b models.ReferralBonus
		if err := rows.Scan(&b.ID, &b.ChainID, &b.PurchaseID, &b.ReferrerID, &b.Amount, &b.Level, &b.Status, &b.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (r *ReferralRepo) TopReferrers(ctx context.Context, limit int) (map[int64]float64, error) {
	const q = `
        SELECT referrer_id, COUNT(*)::float8
        FROM referral_chains
        WHERE created_at >= now() - INTERVAL '30 days' AND level = 1
        GROUP BY referrer_id
        ORDER BY COUNT(*) DESC
        LIMIT $1
    `
	rows, err := r.db.Pool.Query(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]float64{}
	for rows.Next() {
		var uid int64
		var n float64
		if err := rows.Scan(&uid, &n); err != nil {
			return nil, err
		}
		out[uid] = n
	}
	return out, rows.Err()
}

func scanChains(rows pgx.Rows) ([]models.ReferralChain, error) {
	out := make([]models.ReferralChain, 0, 16)
	for rows.Next() {
		var c models.ReferralChain
		if err := rows.Scan(&c.ID, &c.UserID, &c.ReferrerID, &c.Level, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
