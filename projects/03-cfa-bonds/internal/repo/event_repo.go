package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/andrey/cfa-bonds/internal/models"
)

type EventRepo struct {
	pool *pgxpool.Pool
}

func NewEventRepo(pool *pgxpool.Pool) *EventRepo {
	return &EventRepo{pool: pool}
}

func (r *EventRepo) Append(ctx context.Context, q Queryer, e *models.EventLog) error {
	if q == nil {
		q = r.pool
	}
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	payload := e.Payload
	if len(payload) == 0 {
		payload = []byte("{}")
	}
	err := q.QueryRow(ctx, `
		INSERT INTO event_log (id, entity_type, entity_id, event_type, payload)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING created_at`,
		e.ID, e.EntityType, e.EntityID, e.EventType, payload).Scan(&e.CreatedAt)
	if err != nil {
		return fmt.Errorf("append event %s for %s/%s: %w", e.EventType, e.EntityType, e.EntityID, err)
	}
	return nil
}

func (r *EventRepo) ListByEntity(ctx context.Context, entityType string, entityID uuid.UUID, limit int) ([]*models.EventLog, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, entity_type, entity_id, event_type, payload, created_at
		FROM event_log WHERE entity_type=$1 AND entity_id=$2
		ORDER BY created_at DESC LIMIT $3`, entityType, entityID, clampLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("event history for %s/%s: %w", entityType, entityID, err)
	}
	defer rows.Close()
	var out []*models.EventLog
	for rows.Next() {
		var e models.EventLog
		if err := rows.Scan(&e.ID, &e.EntityType, &e.EntityID, &e.EventType, &e.Payload, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		out = append(out, &e)
	}
	return out, rows.Err()
}

func (r *EventRepo) CountByType(ctx context.Context, since time.Time) (map[string]int64, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT event_type, count(*) FROM event_log WHERE created_at >= $1 GROUP BY event_type`, since)
	if err != nil {
		return nil, fmt.Errorf("count events since %s: %w", since.Format(time.RFC3339), err)
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var t string
		var c int64
		if err := rows.Scan(&t, &c); err != nil {
			return nil, fmt.Errorf("scan event count: %w", err)
		}
		out[t] = c
	}
	return out, rows.Err()
}
