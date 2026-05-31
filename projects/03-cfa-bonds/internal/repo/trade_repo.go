package repo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/andrey/cfa-bonds/internal/db"
	"github.com/andrey/cfa-bonds/internal/models"
)

type TradeRepo struct {
	pool *pgxpool.Pool
}

func NewTradeRepo(pool *pgxpool.Pool) *TradeRepo {
	return &TradeRepo{pool: pool}
}

const tradeColumns = `id, issue_id, seller_id, buyer_id, quantity, price, accrued_interest,
	total_amount, status, failure_reason, submitted_at, settled_at`

type TradeFilter struct {
	InvestorID uuid.UUID
	IssueID    uuid.UUID
	Status     string
	Limit      int
	Offset     int
}

func (r *TradeRepo) Create(ctx context.Context, t *models.Trade) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO trades (id, issue_id, seller_id, buyer_id, quantity, price, accrued_interest,
			total_amount, status, failure_reason)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING submitted_at`,
		t.ID, t.IssueID, t.SellerID, t.BuyerID, t.Quantity, db.NumericFromDecimal(t.Price),
		db.NumericFromDecimal(t.AccruedInterest), db.NumericFromDecimal(t.TotalAmount),
		t.Status, t.FailureReason).Scan(&t.SubmittedAt)
	if err != nil {
		return fmt.Errorf("insert trade for issue %s: %w", t.IssueID, err)
	}
	return nil
}

func (r *TradeRepo) Get(ctx context.Context, id uuid.UUID) (*models.Trade, error) {
	row := r.pool.QueryRow(ctx, "SELECT "+tradeColumns+" FROM trades WHERE id=$1", id)
	t, err := scanTrade(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get trade %s: %w", id, err)
	}
	return t, nil
}

func (r *TradeRepo) UpdateStatus(ctx context.Context, q Queryer, id uuid.UUID, status string, ai, total decimal.Decimal, reason string, settled bool) error {
	if q == nil {
		q = r.pool
	}
	var settledExpr string
	if settled {
		settledExpr = "now()"
	} else {
		settledExpr = "settled_at"
	}
	query := fmt.Sprintf(`UPDATE trades SET status=$1, accrued_interest=$2, total_amount=$3,
		failure_reason=$4, settled_at=%s WHERE id=$5`, settledExpr)
	tag, err := q.Exec(ctx, query, status, db.NumericFromDecimal(ai), db.NumericFromDecimal(total), reason, id)
	if err != nil {
		return fmt.Errorf("update trade %s to %s: %w", id, status, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("trade %s vanished during update: %w", id, ErrNotFound)
	}
	return nil
}

func (r *TradeRepo) ListByInvestor(ctx context.Context, investorID uuid.UUID, limit, offset int) ([]*models.Trade, error) {
	rows, err := r.pool.Query(ctx, "SELECT "+tradeColumns+`
		FROM trades WHERE seller_id=$1 OR buyer_id=$1
		ORDER BY submitted_at DESC LIMIT $2 OFFSET $3`,
		investorID, clampLimit(limit), clampOffset(offset))
	if err != nil {
		return nil, fmt.Errorf("trades for investor %s: %w", investorID, err)
	}
	defer rows.Close()
	return collectTrades(rows)
}

func (r *TradeRepo) ListByIssue(ctx context.Context, issueID uuid.UUID, limit, offset int) ([]*models.Trade, error) {
	rows, err := r.pool.Query(ctx, "SELECT "+tradeColumns+`
		FROM trades WHERE issue_id=$1 ORDER BY submitted_at DESC LIMIT $2 OFFSET $3`,
		issueID, clampLimit(limit), clampOffset(offset))
	if err != nil {
		return nil, fmt.Errorf("trades for issue %s: %w", issueID, err)
	}
	defer rows.Close()
	return collectTrades(rows)
}

func (r *TradeRepo) List(ctx context.Context, f TradeFilter) ([]*models.Trade, error) {
	var conds []string
	var args []any
	if f.InvestorID != uuid.Nil {
		args = append(args, f.InvestorID)
		conds = append(conds, fmt.Sprintf("(seller_id=$%d OR buyer_id=$%d)", len(args), len(args)))
	}
	if f.IssueID != uuid.Nil {
		args = append(args, f.IssueID)
		conds = append(conds, fmt.Sprintf("issue_id=$%d", len(args)))
	}
	if f.Status != "" {
		args = append(args, f.Status)
		conds = append(conds, fmt.Sprintf("status=$%d", len(args)))
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	args = append(args, clampLimit(f.Limit), clampOffset(f.Offset))
	query := fmt.Sprintf("SELECT %s FROM trades %s ORDER BY submitted_at DESC LIMIT $%d OFFSET $%d",
		tradeColumns, where, len(args)-1, len(args))
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("filter trades: %w", err)
	}
	defer rows.Close()
	return collectTrades(rows)
}

func (r *TradeRepo) GetVolumeByIssue(ctx context.Context, issueID uuid.UUID, since time.Time) (decimal.Decimal, int64, error) {
	var vol pgxNumeric
	var cnt int64
	err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(total_amount), 0), count(*)
		FROM trades WHERE issue_id=$1 AND status='settled' AND settled_at >= $2`,
		issueID, since).Scan(&vol, &cnt)
	if err != nil {
		return decimal.Zero, 0, fmt.Errorf("volume for issue %s: %w", issueID, err)
	}
	d, err := db.DecimalFromNumeric(vol)
	if err != nil {
		return decimal.Zero, 0, err
	}
	return d, cnt, nil
}

func (r *TradeRepo) GlobalVolumeSince(ctx context.Context, since time.Time) (decimal.Decimal, int64, error) {
	var vol pgxNumeric
	var cnt int64
	err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(total_amount), 0), count(*)
		FROM trades WHERE status='settled' AND settled_at >= $1`, since).Scan(&vol, &cnt)
	if err != nil {
		return decimal.Zero, 0, fmt.Errorf("global volume since %s: %w", since.Format(time.RFC3339), err)
	}
	d, err := db.DecimalFromNumeric(vol)
	if err != nil {
		return decimal.Zero, 0, err
	}
	return d, cnt, nil
}

func (r *TradeRepo) LastPrice(ctx context.Context, issueID uuid.UUID) (decimal.Decimal, bool, error) {
	var price pgxNumeric
	err := r.pool.QueryRow(ctx, `
		SELECT price FROM trades WHERE issue_id=$1 AND status='settled'
		ORDER BY settled_at DESC NULLS LAST LIMIT 1`, issueID).Scan(&price)
	if errors.Is(err, pgx.ErrNoRows) {
		return decimal.Zero, false, nil
	}
	if err != nil {
		return decimal.Zero, false, fmt.Errorf("last price for %s: %w", issueID, err)
	}
	d, err := db.DecimalFromNumeric(price)
	if err != nil {
		return decimal.Zero, false, err
	}
	return d, true, nil
}

func (r *TradeRepo) CountSettled(ctx context.Context) (int64, decimal.Decimal, error) {
	var cnt int64
	var vol pgxNumeric
	err := r.pool.QueryRow(ctx, `
		SELECT count(*), COALESCE(SUM(total_amount),0) FROM trades WHERE status='settled'`).Scan(&cnt, &vol)
	if err != nil {
		return 0, decimal.Zero, fmt.Errorf("count settled trades: %w", err)
	}
	d, err := db.DecimalFromNumeric(vol)
	if err != nil {
		return 0, decimal.Zero, err
	}
	return cnt, d, nil
}

func scanTrade(row pgx.Row) (*models.Trade, error) {
	var t models.Trade
	var price, ai, total pgxNumeric
	err := row.Scan(&t.ID, &t.IssueID, &t.SellerID, &t.BuyerID, &t.Quantity, &price, &ai, &total,
		&t.Status, &t.FailureReason, &t.SubmittedAt, &t.SettledAt)
	if err != nil {
		return nil, err
	}
	if t.Price, err = db.DecimalFromNumeric(price); err != nil {
		return nil, fmt.Errorf("decode price: %w", err)
	}
	if t.AccruedInterest, err = db.DecimalFromNumeric(ai); err != nil {
		return nil, fmt.Errorf("decode accrued: %w", err)
	}
	if t.TotalAmount, err = db.DecimalFromNumeric(total); err != nil {
		return nil, fmt.Errorf("decode total: %w", err)
	}
	return &t, nil
}

func collectTrades(rows pgx.Rows) ([]*models.Trade, error) {
	var out []*models.Trade
	for rows.Next() {
		t, err := scanTrade(rows)
		if err != nil {
			return nil, fmt.Errorf("scan trade: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
