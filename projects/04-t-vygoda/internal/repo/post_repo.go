package repo

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/andrey/t-vygoda/internal/db"
	"github.com/andrey/t-vygoda/internal/models"
)

type PostRepo struct {
	db *db.DB
}

func NewPostRepo(d *db.DB) *PostRepo { return &PostRepo{db: d} }

type CreatePost struct {
	UserID      int64
	Title       string
	Description string
	ImageURL    *string
	PriceBefore *float64
	PriceAfter  *float64
	PromoID     *int64
	CategoryID  *int64
}

func (r *PostRepo) Create(ctx context.Context, in CreatePost) (*models.Post, error) {
	const q = `
        INSERT INTO posts (user_id, title, description, image_url, price_before, price_after, promo_id, category_id)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
        RETURNING id, user_id, title, description, image_url, price_before, price_after, promo_id, category_id, likes_count, created_at
    `
	var p models.Post
	err := r.db.Pool.QueryRow(ctx, q,
		in.UserID, in.Title, in.Description, in.ImageURL,
		in.PriceBefore, in.PriceAfter, in.PromoID, in.CategoryID,
	).Scan(
		&p.ID, &p.UserID, &p.Title, &p.Description, &p.ImageURL,
		&p.PriceBefore, &p.PriceAfter, &p.PromoID, &p.CategoryID,
		&p.LikesCount, &p.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert post: %w", err)
	}
	return &p, nil
}

func (r *PostRepo) GetByID(ctx context.Context, id int64, viewerID int64) (*models.PostWithUser, error) {
	const q = `
        SELECT p.id, p.user_id, p.title, p.description, p.image_url, p.price_before, p.price_after,
               p.promo_id, p.category_id, p.likes_count, p.created_at,
               u.id, u.name, u.avatar_url, u.level,
               EXISTS(SELECT 1 FROM post_likes WHERE post_id = p.id AND user_id = $2)
        FROM posts p
        JOIN users u ON u.id = p.user_id
        WHERE p.id = $1
    `
	var p models.PostWithUser
	err := r.db.Pool.QueryRow(ctx, q, id, viewerID).Scan(
		&p.ID, &p.UserID, &p.Title, &p.Description, &p.ImageURL,
		&p.PriceBefore, &p.PriceAfter, &p.PromoID, &p.CategoryID,
		&p.LikesCount, &p.CreatedAt,
		&p.Author.ID, &p.Author.Name, &p.Author.AvatarURL, &p.Author.Level,
		&p.Liked,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get post: %w", err)
	}
	return &p, nil
}

type FeedQuery struct {
	ViewerID   int64
	CategoryID *int64
	Sort       models.FeedSort
	PromoIDs   []int64
	Limit      int
	Offset     int
}

func (r *PostRepo) GetFeed(ctx context.Context, q FeedQuery) ([]models.PostWithUser, error) {
	if q.Limit <= 0 || q.Limit > 100 {
		q.Limit = 20
	}
	args := []any{q.ViewerID}
	sql := strings.Builder{}
	sql.WriteString(`
        SELECT p.id, p.user_id, p.title, p.description, p.image_url, p.price_before, p.price_after,
               p.promo_id, p.category_id, p.likes_count, p.created_at,
               u.id, u.name, u.avatar_url, u.level,
               EXISTS(SELECT 1 FROM post_likes WHERE post_id = p.id AND user_id = $1) AS liked
        FROM posts p
        JOIN users u ON u.id = p.user_id
        WHERE 1=1
    `)
	if q.CategoryID != nil {
		args = append(args, *q.CategoryID)
		sql.WriteString(fmt.Sprintf(" AND p.category_id = $%d", len(args)))
	}
	if q.Sort == models.FeedSortRecommended && len(q.PromoIDs) > 0 {
		args = append(args, q.PromoIDs)
		sql.WriteString(fmt.Sprintf(" AND p.promo_id = ANY($%d::bigint[])", len(args)))
	}
	switch q.Sort {
	case models.FeedSortPopular:
		sql.WriteString(" ORDER BY p.likes_count DESC, p.created_at DESC ")
	case models.FeedSortRecommended:
		if len(q.PromoIDs) > 0 {
			sql.WriteString(" ORDER BY array_position($")
			sql.WriteString(strconv.Itoa(len(args)))
			sql.WriteString("::bigint[], p.promo_id), p.created_at DESC ")
		} else {
			sql.WriteString(" ORDER BY p.likes_count DESC, p.created_at DESC ")
		}
	default:
		sql.WriteString(" ORDER BY p.created_at DESC ")
	}
	args = append(args, q.Limit, q.Offset)
	sql.WriteString(fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)-1, len(args)))

	rows, err := r.db.Pool.Query(ctx, sql.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("feed query: %w", err)
	}
	defer rows.Close()
	return scanFeed(rows)
}

func (r *PostRepo) GetByUser(ctx context.Context, userID, viewerID int64, limit, offset int) ([]models.PostWithUser, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	const q = `
        SELECT p.id, p.user_id, p.title, p.description, p.image_url, p.price_before, p.price_after,
               p.promo_id, p.category_id, p.likes_count, p.created_at,
               u.id, u.name, u.avatar_url, u.level,
               EXISTS(SELECT 1 FROM post_likes WHERE post_id = p.id AND user_id = $2) AS liked
        FROM posts p
        JOIN users u ON u.id = p.user_id
        WHERE p.user_id = $1
        ORDER BY p.created_at DESC
        LIMIT $3 OFFSET $4
    `
	rows, err := r.db.Pool.Query(ctx, q, userID, viewerID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFeed(rows)
}

func (r *PostRepo) Like(ctx context.Context, postID, userID int64) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	ct, err := tx.Exec(ctx, `INSERT INTO post_likes (user_id, post_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, postID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return ErrNotFound
		}
		return fmt.Errorf("like insert: %w", err)
	}
	if ct.RowsAffected() > 0 {
		if _, err := tx.Exec(ctx, `UPDATE posts SET likes_count = likes_count + 1 WHERE id = $1`, postID); err != nil {
			return fmt.Errorf("like incr: %w", err)
		}
	}
	return tx.Commit(ctx)
}

func (r *PostRepo) Unlike(ctx context.Context, postID, userID int64) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	ct, err := tx.Exec(ctx, `DELETE FROM post_likes WHERE user_id = $1 AND post_id = $2`, userID, postID)
	if err != nil {
		return fmt.Errorf("unlike: %w", err)
	}
	if ct.RowsAffected() > 0 {
		if _, err := tx.Exec(ctx, `UPDATE posts SET likes_count = GREATEST(likes_count - 1, 0) WHERE id = $1`, postID); err != nil {
			return fmt.Errorf("unlike decr: %w", err)
		}
	}
	return tx.Commit(ctx)
}

func (r *PostRepo) LikedByUser(ctx context.Context, userID int64, limit, offset int) ([]models.PostWithUser, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	const q = `
        SELECT p.id, p.user_id, p.title, p.description, p.image_url, p.price_before, p.price_after,
               p.promo_id, p.category_id, p.likes_count, p.created_at,
               u.id, u.name, u.avatar_url, u.level, TRUE
        FROM post_likes l
        JOIN posts p ON p.id = l.post_id
        JOIN users u ON u.id = p.user_id
        WHERE l.user_id = $1
        ORDER BY l.created_at DESC
        LIMIT $2 OFFSET $3
    `
	rows, err := r.db.Pool.Query(ctx, q, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFeed(rows)
}

func (r *PostRepo) CountByCategory(ctx context.Context) (map[int64]int64, error) {
	rows, err := r.db.Pool.Query(ctx, `
        SELECT COALESCE(category_id, 0), count(*)
        FROM posts GROUP BY category_id
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]int64{}
	for rows.Next() {
		var id, n int64
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

func scanFeed(rows pgx.Rows) ([]models.PostWithUser, error) {
	out := make([]models.PostWithUser, 0, 32)
	for rows.Next() {
		var p models.PostWithUser
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.Title, &p.Description, &p.ImageURL,
			&p.PriceBefore, &p.PriceAfter, &p.PromoID, &p.CategoryID,
			&p.LikesCount, &p.CreatedAt,
			&p.Author.ID, &p.Author.Name, &p.Author.AvatarURL, &p.Author.Level,
			&p.Liked,
		); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
