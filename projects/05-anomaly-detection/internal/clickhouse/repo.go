package clickhouse

import (
	"context"
	"fmt"
	"time"

	"github.com/andrey/anomaly-detection/internal/models"
)

type AlertRepo struct {
	c *Client
}

func NewAlertRepo(c *Client) *AlertRepo { return &AlertRepo{c: c} }

func (r *AlertRepo) InsertAlerts(ctx context.Context, alerts []models.Alert) error {
	if len(alerts) == 0 {
		return nil
	}
	batch, err := r.c.conn.PrepareBatch(ctx, "INSERT INTO anomalies.alerts")
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}
	for _, a := range alerts {
		flag := uint8(0)
		if a.IforestFlag {
			flag = 1
		}
		if err := batch.Append(
			a.ID, a.TxID, a.ClientID, a.Score,
			flag, a.AutoencoderScore, string(a.Level), a.CreatedAt,
		); err != nil {
			return fmt.Errorf("append: %w", err)
		}
	}
	return batch.Send()
}

func (r *AlertRepo) GetRecent(ctx context.Context, limit int) ([]models.Alert, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	const q = `
        SELECT id, tx_id, client_id, score, iforest_flag, autoencoder_score, level, created_at
        FROM anomalies.alerts
        ORDER BY created_at DESC LIMIT ?
    `
	rows, err := r.c.conn.Query(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAlerts(rows, limit)
}

func (r *AlertRepo) GetByClient(ctx context.Context, clientID string, limit int) ([]models.Alert, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	const q = `
        SELECT id, tx_id, client_id, score, iforest_flag, autoencoder_score, level, created_at
        FROM anomalies.alerts
        WHERE client_id = ?
        ORDER BY created_at DESC LIMIT ?
    `
	rows, err := r.c.conn.Query(ctx, q, clientID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAlerts(rows, limit)
}

type AlertStats struct {
	Total      uint64             `json:"total"`
	ByLevel    map[string]uint64  `json:"by_level"`
	AvgScore   float64            `json:"avg_score"`
	WindowFrom time.Time          `json:"window_from"`
	WindowTo   time.Time          `json:"window_to"`
}

func (r *AlertRepo) GetStats(ctx context.Context, window time.Duration) (*AlertStats, error) {
	if window <= 0 {
		window = time.Hour
	}
	to := time.Now().UTC()
	from := to.Add(-window)
	const q = `
        SELECT level, count(), avg(score)
        FROM anomalies.alerts
        WHERE created_at >= ? AND created_at < ?
        GROUP BY level
    `
	rows, err := r.c.conn.Query(ctx, q, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := &AlertStats{ByLevel: map[string]uint64{}, WindowFrom: from, WindowTo: to}
	var totalScore float64
	var totalCount uint64
	for rows.Next() {
		var level string
		var n uint64
		var avg float64
		if err := rows.Scan(&level, &n, &avg); err != nil {
			return nil, err
		}
		out.ByLevel[level] = n
		totalCount += n
		totalScore += avg * float64(n)
	}
	out.Total = totalCount
	if totalCount > 0 {
		out.AvgScore = totalScore / float64(totalCount)
	}
	return out, rows.Err()
}

func scanAlerts(rows interface {
	Next() bool
	Err() error
	Scan(...any) error
}, capHint int) ([]models.Alert, error) {
	out := make([]models.Alert, 0, capHint)
	for rows.Next() {
		var a models.Alert
		var flag uint8
		var level string
		if err := rows.Scan(
			&a.ID, &a.TxID, &a.ClientID, &a.Score,
			&flag, &a.AutoencoderScore, &level, &a.CreatedAt,
		); err != nil {
			return nil, err
		}
		a.IforestFlag = flag == 1
		a.Level = models.AlertLevel(level)
		out = append(out, a)
	}
	return out, rows.Err()
}
