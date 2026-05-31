package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/andrey/t-vygoda/internal/db"
	"github.com/andrey/t-vygoda/internal/models"
)

type CategoryRepo struct {
	db *db.DB
}

func NewCategoryRepo(d *db.DB) *CategoryRepo { return &CategoryRepo{db: d} }

func (r *CategoryRepo) GetAll(ctx context.Context) ([]models.Category, error) {
	rows, err := r.db.Pool.Query(ctx, `
        SELECT id, name, slug, parent_id, created_at
        FROM categories ORDER BY name ASC
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.Category, 0, 16)
	for rows.Next() {
		var c models.Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.ParentID, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *CategoryRepo) GetBySlug(ctx context.Context, slug string) (*models.Category, error) {
	const q = `SELECT id, name, slug, parent_id, created_at FROM categories WHERE slug = $1`
	var c models.Category
	err := r.db.Pool.QueryRow(ctx, q, slug).Scan(&c.ID, &c.Name, &c.Slug, &c.ParentID, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get category: %w", err)
	}
	return &c, nil
}

func (r *CategoryRepo) GetTree(ctx context.Context) ([]models.CategoryNode, error) {
	all, err := r.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]*models.CategoryNode, len(all))
	for i := range all {
		n := models.CategoryNode{Category: all[i]}
		byID[all[i].ID] = &n
	}
	var roots []models.CategoryNode
	for _, n := range byID {
		if n.ParentID == nil {
			roots = append(roots, *n)
		}
	}
	for _, n := range byID {
		if n.ParentID != nil {
			if parent, ok := byID[*n.ParentID]; ok {
				parent.Children = append(parent.Children, *n)
			}
		}
	}
	for i, r := range roots {
		if found, ok := byID[r.ID]; ok {
			roots[i].Children = found.Children
		}
	}
	return roots, nil
}
