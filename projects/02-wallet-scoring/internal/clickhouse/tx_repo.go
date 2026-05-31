package clickhouse

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/andrey/wallet-scoring/internal/models"
)

var ErrNotFound = errors.New("not found")

type TxRepo struct {
	c *Client
}

func NewTxRepo(c *Client) *TxRepo { return &TxRepo{c: c} }

func (r *TxRepo) InsertBatch(ctx context.Context, txs []models.Transaction) error {
	if len(txs) == 0 {
		return nil
	}
	start := time.Now()
	batch, err := r.c.conn.PrepareBatch(ctx, "INSERT INTO wallets.transactions")
	if err != nil {
		r.c.recordLatency(start, err)
		return fmt.Errorf("prepare batch: %w", err)
	}
	for _, t := range txs {
		var success uint8
		if t.Success {
			success = 1
		}
		if err := batch.Append(
			t.Signature, t.Slot, t.BlockTime, t.Fee,
			t.Sender, t.Receiver, t.Amount, t.ProgramID,
			t.SwapKind, success, t.Accounts, t.RawData,
		); err != nil {
			r.c.recordLatency(start, err)
			return fmt.Errorf("append: %w", err)
		}
	}
	err = batch.Send()
	r.c.recordLatency(start, err)
	return err
}

func (r *TxRepo) GetByWallet(ctx context.Context, addr string, limit int, cursor string) ([]models.Transaction, string, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	args := []any{addr, addr}
	q := strings.Builder{}
	q.WriteString(`
        SELECT signature, slot, block_time, fee, sender, receiver, amount,
               program_id, swap_kind, success, raw_accounts, raw_data
        FROM wallets.transactions
        WHERE (sender = ? OR receiver = ?)
    `)
	if cursor != "" {
		slot, sig, err := parseCursor(cursor)
		if err != nil {
			return nil, "", fmt.Errorf("invalid cursor: %w", err)
		}
		q.WriteString(" AND (slot < ? OR (slot = ? AND signature < ?)) ")
		args = append(args, slot, slot, sig)
	}
	q.WriteString(" ORDER BY slot DESC, signature DESC LIMIT ? ")
	args = append(args, limit)

	start := time.Now()
	rows, err := r.c.conn.Query(ctx, q.String(), args...)
	if err != nil {
		r.c.recordLatency(start, err)
		return nil, "", fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	out, err := scanTxRows(rows, limit)
	r.c.recordLatency(start, err)
	if err != nil {
		return nil, "", err
	}

	var next string
	if len(out) == limit {
		last := out[len(out)-1]
		next = formatCursor(last.Slot, last.Signature)
	}
	return out, next, nil
}

func (r *TxRepo) GetBySignature(ctx context.Context, sig string) (*models.Transaction, error) {
	const q = `
        SELECT signature, slot, block_time, fee, sender, receiver, amount,
               program_id, swap_kind, success, raw_accounts, raw_data
        FROM wallets.transactions WHERE signature = ? LIMIT 1
    `
	start := time.Now()
	row := r.c.conn.QueryRow(ctx, q, sig)
	t, err := scanOneTx(row)
	r.c.recordLatency(start, err)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (r *TxRepo) GetRecentByProgram(ctx context.Context, programID string, limit int) ([]models.Transaction, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	const q = `
        SELECT signature, slot, block_time, fee, sender, receiver, amount,
               program_id, swap_kind, success, raw_accounts, raw_data
        FROM wallets.transactions
        WHERE program_id = ?
        ORDER BY block_time DESC, slot DESC
        LIMIT ?
    `
	start := time.Now()
	rows, err := r.c.conn.Query(ctx, q, programID, limit)
	if err != nil {
		r.c.recordLatency(start, err)
		return nil, err
	}
	defer rows.Close()
	out, err := scanTxRows(rows, limit)
	r.c.recordLatency(start, err)
	return out, err
}

func (r *TxRepo) GetByTimeRange(ctx context.Context, from, to time.Time, limit int, cursor string) ([]models.Transaction, string, error) {
	if limit <= 0 || limit > 10000 {
		limit = 1000
	}
	args := []any{from, to}
	q := strings.Builder{}
	q.WriteString(`
        SELECT signature, slot, block_time, fee, sender, receiver, amount,
               program_id, swap_kind, success, raw_accounts, raw_data
        FROM wallets.transactions
        WHERE block_time >= ? AND block_time < ?
    `)
	if cursor != "" {
		slot, sig, err := parseCursor(cursor)
		if err != nil {
			return nil, "", fmt.Errorf("invalid cursor: %w", err)
		}
		q.WriteString(" AND (slot > ? OR (slot = ? AND signature > ?)) ")
		args = append(args, slot, slot, sig)
	}
	q.WriteString(" ORDER BY slot ASC, signature ASC LIMIT ? ")
	args = append(args, limit)

	start := time.Now()
	rows, err := r.c.conn.Query(ctx, q.String(), args...)
	if err != nil {
		r.c.recordLatency(start, err)
		return nil, "", err
	}
	defer rows.Close()
	out, err := scanTxRows(rows, limit)
	r.c.recordLatency(start, err)
	if err != nil {
		return nil, "", err
	}
	var next string
	if len(out) == limit {
		last := out[len(out)-1]
		next = formatCursor(last.Slot, last.Signature)
	}
	return out, next, nil
}

func (r *TxRepo) CountByTimeRange(ctx context.Context, from, to time.Time) (uint64, error) {
	var n uint64
	start := time.Now()
	err := r.c.conn.QueryRow(ctx, `
        SELECT count() FROM wallets.transactions
        WHERE block_time >= ? AND block_time < ?
    `, from, to).Scan(&n)
	r.c.recordLatency(start, err)
	return n, err
}

func (r *TxRepo) GetVolumeByWallet(ctx context.Context, addr string, from time.Time) (uint64, error) {
	var v uint64
	start := time.Now()
	err := r.c.conn.QueryRow(ctx, `
        SELECT sum(amount) FROM wallets.transactions
        WHERE (sender = ? OR receiver = ?) AND block_time >= ?
    `, addr, addr, from).Scan(&v)
	r.c.recordLatency(start, err)
	return v, err
}

func scanTxRows(rows interface {
	Next() bool
	Err() error
	Scan(...any) error
}, capHint int) ([]models.Transaction, error) {
	out := make([]models.Transaction, 0, capHint)
	for rows.Next() {
		var t models.Transaction
		var success uint8
		if err := rows.Scan(
			&t.Signature, &t.Slot, &t.BlockTime, &t.Fee,
			&t.Sender, &t.Receiver, &t.Amount, &t.ProgramID,
			&t.SwapKind, &success, &t.Accounts, &t.RawData,
		); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		t.Success = success == 1
		out = append(out, t)
	}
	return out, rows.Err()
}

func scanOneTx(row interface{ Scan(...any) error }) (*models.Transaction, error) {
	var t models.Transaction
	var success uint8
	if err := row.Scan(
		&t.Signature, &t.Slot, &t.BlockTime, &t.Fee,
		&t.Sender, &t.Receiver, &t.Amount, &t.ProgramID,
		&t.SwapKind, &success, &t.Accounts, &t.RawData,
	); err != nil {
		return nil, err
	}
	t.Success = success == 1
	return &t, nil
}

func formatCursor(slot uint64, sig string) string {
	return strconv.FormatUint(slot, 10) + ":" + sig
}

func parseCursor(c string) (uint64, string, error) {
	parts := strings.SplitN(c, ":", 2)
	if len(parts) != 2 {
		return 0, "", fmt.Errorf("malformed cursor")
	}
	slot, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return 0, "", err
	}
	return slot, parts[1], nil
}
