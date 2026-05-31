package repo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/andrey/t-vygoda/internal/db"
	"github.com/andrey/t-vygoda/internal/models"
)

type UserRepo struct {
	db *db.DB
}

func NewUserRepo(d *db.DB) *UserRepo { return &UserRepo{db: d} }

type CreateUserInput struct {
	Phone        string
	Name         string
	Email        *string
	ReferralCode string
	ReferredBy   *int64
}

func (r *UserRepo) Create(ctx context.Context, in CreateUserInput) (*models.User, error) {
	code := in.ReferralCode
	if code == "" {
		code = newReferralCode()
	}
	const q = `
        INSERT INTO users (phone, name, email, referral_code, referred_by)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING id, phone, name, email, avatar_url, referral_code, referred_by, level, created_at, updated_at
    `
	var u models.User
	err := r.db.Pool.QueryRow(ctx, q, in.Phone, in.Name, in.Email, code, in.ReferredBy).Scan(
		&u.ID, &u.Phone, &u.Name, &u.Email, &u.AvatarURL,
		&u.ReferralCode, &u.ReferredBy, &u.Level, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrDuplicate
		}
		return nil, fmt.Errorf("insert user: %w", err)
	}
	return &u, nil
}

func (r *UserRepo) GetByID(ctx context.Context, id int64) (*models.User, error) {
	return r.scanOne(ctx, "WHERE id = $1", id)
}

func (r *UserRepo) GetByPhone(ctx context.Context, phone string) (*models.User, error) {
	return r.scanOne(ctx, "WHERE phone = $1", phone)
}

func (r *UserRepo) GetByReferralCode(ctx context.Context, code string) (*models.User, error) {
	return r.scanOne(ctx, "WHERE referral_code = $1", code)
}

func (r *UserRepo) Update(ctx context.Context, id int64, name string, email, avatarURL *string) (*models.User, error) {
	const q = `
        UPDATE users SET name = $2, email = $3, avatar_url = $4, updated_at = now()
        WHERE id = $1
        RETURNING id, phone, name, email, avatar_url, referral_code, referred_by, level, created_at, updated_at
    `
	var u models.User
	err := r.db.Pool.QueryRow(ctx, q, id, name, email, avatarURL).Scan(
		&u.ID, &u.Phone, &u.Name, &u.Email, &u.AvatarURL,
		&u.ReferralCode, &u.ReferredBy, &u.Level, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}
	return &u, nil
}

func (r *UserRepo) Search(ctx context.Context, query string, limit int) ([]models.User, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	const q = `
        SELECT id, phone, name, email, avatar_url, referral_code, referred_by, level, created_at, updated_at
        FROM users
        WHERE name ILIKE $1 OR phone ILIKE $1
        ORDER BY name ASC
        LIMIT $2
    `
	rows, err := r.db.Pool.Query(ctx, q, "%"+query+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanUsers(rows)
}

func (r *UserRepo) TopByPurchases(ctx context.Context, limit int) ([]models.User, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	const q = `
        SELECT u.id, u.phone, u.name, u.email, u.avatar_url, u.referral_code, u.referred_by, u.level, u.created_at, u.updated_at
        FROM users u
        JOIN (
            SELECT user_id, SUM(amount) AS total
            FROM purchases
            WHERE status = 'confirmed' AND created_at >= now() - INTERVAL '30 days'
            GROUP BY user_id
        ) p ON p.user_id = u.id
        ORDER BY p.total DESC
        LIMIT $1
    `
	rows, err := r.db.Pool.Query(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanUsers(rows)
}

func (r *UserRepo) Stats(ctx context.Context, userID int64) (*models.UserStats, error) {
	const q = `
        SELECT
            (SELECT COUNT(*) FROM posts WHERE user_id = $1),
            (SELECT COUNT(*) FROM purchases WHERE user_id = $1 AND status = 'confirmed'),
            COALESCE((SELECT SUM(amount) FROM purchases WHERE user_id = $1 AND status = 'confirmed'), 0),
            (SELECT COUNT(*) FROM referral_chains WHERE referrer_id = $1 AND level = 1),
            (SELECT COUNT(*) FROM referral_chains WHERE referrer_id = $1),
            COALESCE((SELECT SUM(amount) FROM referral_bonuses WHERE referrer_id = $1), 0)
    `
	s := &models.UserStats{UserID: userID}
	err := r.db.Pool.QueryRow(ctx, q, userID).Scan(
		&s.PostsCount, &s.PurchasesCount, &s.TotalSpent,
		&s.ReferralsLevel1, &s.ReferralsTotal, &s.BonusesEarned,
	)
	if err != nil {
		return nil, fmt.Errorf("user stats: %w", err)
	}
	return s, nil
}

func (r *UserRepo) Count(ctx context.Context) (uint64, error) {
	var n uint64
	if err := r.db.Pool.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&n); err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return n, nil
}

func (r *UserRepo) GenerateUniqueReferralCode(ctx context.Context) (string, error) {
	for i := 0; i < 5; i++ {
		code := newReferralCode()
		var exists bool
		err := r.db.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE referral_code = $1)", code).Scan(&exists)
		if err != nil {
			return "", err
		}
		if !exists {
			return code, nil
		}
	}
	return "", errors.New("referral code generation exhausted")
}

func (r *UserRepo) scanOne(ctx context.Context, where string, args ...any) (*models.User, error) {
	q := `
        SELECT id, phone, name, email, avatar_url, referral_code, referred_by, level, created_at, updated_at
        FROM users ` + where + ` LIMIT 1`
	var u models.User
	err := r.db.Pool.QueryRow(ctx, q, args...).Scan(
		&u.ID, &u.Phone, &u.Name, &u.Email, &u.AvatarURL,
		&u.ReferralCode, &u.ReferredBy, &u.Level, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return &u, nil
}

func scanUsers(rows pgx.Rows) ([]models.User, error) {
	out := make([]models.User, 0, 16)
	for rows.Next() {
		var u models.User
		if err := rows.Scan(
			&u.ID, &u.Phone, &u.Name, &u.Email, &u.AvatarURL,
			&u.ReferralCode, &u.ReferredBy, &u.Level, &u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func newReferralCode() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return strings.ToUpper(hex.EncodeToString(b[:]))
}
