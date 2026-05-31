package clickhouse

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/andrey/portfolio-reports/internal/models"
)

var ErrNotFound = errors.New("not found")

type TokenBalance struct {
	Mint      string
	Symbol    string
	Amount    float64
	LastPrice float64
}

type WalletScore struct {
	Wallet    string
	Score     float32
	Category  string
	UpdatedAt time.Time
}

type TokenScore struct {
	Mint     string
	Category string
}

type Repo struct {
	c *Client
}

func NewRepo(c *Client) *Repo { return &Repo{c: c} }

func (r *Repo) GetTokenBalances(ctx context.Context, address string) ([]TokenBalance, error) {
	const q = `
        SELECT mint, symbol, amount, last_price
        FROM wallets.token_balances FINAL
        WHERE wallet = ? AND amount > 0
        ORDER BY (amount * last_price) DESC
    `
	rows, err := r.c.conn.Query(ctx, q, address)
	if err != nil {
		return nil, fmt.Errorf("query balances: %w", err)
	}
	defer rows.Close()
	out := make([]TokenBalance, 0, 16)
	for rows.Next() {
		var b TokenBalance
		if err := rows.Scan(&b.Mint, &b.Symbol, &b.Amount, &b.LastPrice); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (r *Repo) GetPnLByToken(ctx context.Context, address string) ([]models.TokenPnL, error) {
	const q = `
        SELECT mint, avg_buy_price, current_price, quantity, realized_pnl, unrealized_pnl
        FROM wallets.token_pnl FINAL
        WHERE wallet = ?
    `
	rows, err := r.c.conn.Query(ctx, q, address)
	if err != nil {
		return nil, fmt.Errorf("query pnl: %w", err)
	}
	defer rows.Close()
	out := make([]models.TokenPnL, 0, 16)
	for rows.Next() {
		var p models.TokenPnL
		if err := rows.Scan(&p.Mint, &p.AvgBuyPrice, &p.CurrentPrice, &p.Quantity, &p.RealizedPnL, &p.UnrealizedPnL); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *Repo) GetWalletScore(ctx context.Context, address string) (*WalletScore, error) {
	const q = `
        SELECT wallet, score, category, updated_at
        FROM wallets.wallet_stats FINAL
        WHERE wallet = ? LIMIT 1
    `
	var w WalletScore
	err := r.c.conn.QueryRow(ctx, q, address).Scan(&w.Wallet, &w.Score, &w.Category, &w.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *Repo) GetTokenScores(ctx context.Context, mints []string) (map[string]TokenScore, error) {
	if len(mints) == 0 {
		return map[string]TokenScore{}, nil
	}
	const q = `
        SELECT mint, category
        FROM wallets.token_scores FINAL
        WHERE mint IN ?
    `
	rows, err := r.c.conn.Query(ctx, q, mints)
	if err != nil {
		return nil, fmt.Errorf("query token scores: %w", err)
	}
	defer rows.Close()
	out := make(map[string]TokenScore, len(mints))
	for rows.Next() {
		var s TokenScore
		if err := rows.Scan(&s.Mint, &s.Category); err != nil {
			return nil, err
		}
		out[s.Mint] = s
	}
	return out, rows.Err()
}

func (r *Repo) UpsertTokenBalances(ctx context.Context, address string, balances []TokenBalance) error {
	if len(balances) == 0 {
		return nil
	}
	batch, err := r.c.conn.PrepareBatch(ctx, "INSERT INTO wallets.token_balances")
	if err != nil {
		return fmt.Errorf("prepare balances batch: %w", err)
	}
	now := time.Now().UTC()
	for _, b := range balances {
		if err := batch.Append(address, b.Mint, b.Symbol, b.Amount, b.LastPrice, now); err != nil {
			return fmt.Errorf("append balance: %w", err)
		}
	}
	return batch.Send()
}

func (r *Repo) UpsertPnL(ctx context.Context, address string, items []models.TokenPnL) error {
	if len(items) == 0 {
		return nil
	}
	batch, err := r.c.conn.PrepareBatch(ctx, "INSERT INTO wallets.token_pnl")
	if err != nil {
		return fmt.Errorf("prepare pnl batch: %w", err)
	}
	now := time.Now().UTC()
	for _, p := range items {
		if err := batch.Append(address, p.Mint, p.AvgBuyPrice, p.CurrentPrice, p.Quantity, p.RealizedPnL, p.UnrealizedPnL, now); err != nil {
			return fmt.Errorf("append pnl: %w", err)
		}
	}
	return batch.Send()
}

func (r *Repo) UpsertTokenScores(ctx context.Context, scores []TokenScore) error {
	if len(scores) == 0 {
		return nil
	}
	batch, err := r.c.conn.PrepareBatch(ctx, "INSERT INTO wallets.token_scores")
	if err != nil {
		return fmt.Errorf("prepare token scores: %w", err)
	}
	now := time.Now().UTC()
	for _, s := range scores {
		if err := batch.Append(s.Mint, s.Category, float32(0.8), float32(50.0), uint32(0), float64(0), now); err != nil {
			return fmt.Errorf("append token score: %w", err)
		}
	}
	return batch.Send()
}
