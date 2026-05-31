package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/andrey/cfa-bonds/internal/db"
	"github.com/andrey/cfa-bonds/internal/models"
)

type CouponRepo struct {
	pool *pgxpool.Pool
}

func NewCouponRepo(pool *pgxpool.Pool) *CouponRepo {
	return &CouponRepo{pool: pool}
}

func (r *CouponRepo) CreateBatch(ctx context.Context, q Queryer, schedule []*models.CouponSchedule) error {
	if q == nil {
		q = r.pool
	}
	if len(schedule) == 0 {
		return fmt.Errorf("empty coupon schedule")
	}
	batch := &pgx.Batch{}
	for _, c := range schedule {
		if c.ID == uuid.Nil {
			c.ID = uuid.New()
		}
		batch.Queue(`
			INSERT INTO coupon_schedule (id, issue_id, sequence_num, payment_date, amount, status)
			VALUES ($1,$2,$3,$4,$5,$6)`,
			c.ID, c.IssueID, c.SequenceNum, c.PaymentDate, db.NumericFromDecimal(c.Amount), c.Status)
	}
	br := q.(interface {
		SendBatch(context.Context, *pgx.Batch) pgx.BatchResults
	}).SendBatch(ctx, batch)
	defer br.Close()
	for range schedule {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("insert coupon row: %w", err)
		}
	}
	return nil
}

func (r *CouponRepo) GetNextDue(ctx context.Context, q Queryer, issueID uuid.UUID) (*models.CouponSchedule, error) {
	if q == nil {
		q = r.pool
	}
	row := q.QueryRow(ctx, `
		SELECT id, issue_id, sequence_num, payment_date, amount, status, paid_at
		FROM coupon_schedule
		WHERE issue_id=$1 AND status=$2
		ORDER BY payment_date ASC LIMIT 1`, issueID, models.CouponScheduled)
	c, err := scanCoupon(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("next due coupon for %s: %w", issueID, err)
	}
	return c, nil
}

func (r *CouponRepo) MarkPaid(ctx context.Context, q Queryer, id uuid.UUID) error {
	if q == nil {
		q = r.pool
	}
	tag, err := q.Exec(ctx, `
		UPDATE coupon_schedule SET status=$1, paid_at=now()
		WHERE id=$2 AND status IN ($3,$4)`,
		models.CouponPaid, id, models.CouponScheduled, models.CouponProcessing)
	if err != nil {
		return fmt.Errorf("mark coupon %s paid: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("coupon %s already finalized: %w", id, ErrConflict)
	}
	return nil
}

func (r *CouponRepo) GetByIssue(ctx context.Context, issueID uuid.UUID) ([]*models.CouponSchedule, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, issue_id, sequence_num, payment_date, amount, status, paid_at
		FROM coupon_schedule WHERE issue_id=$1 ORDER BY sequence_num`, issueID)
	if err != nil {
		return nil, fmt.Errorf("coupon schedule for %s: %w", issueID, err)
	}
	defer rows.Close()
	var out []*models.CouponSchedule
	for rows.Next() {
		c, err := scanCoupon(rows)
		if err != nil {
			return nil, fmt.Errorf("scan coupon: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *CouponRepo) GetPaidByInvestor(ctx context.Context, investorID uuid.UUID) (decimal.Decimal, error) {
	var total pgxNumeric
	err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount),0) FROM coupon_payments WHERE investor_id=$1`, investorID).Scan(&total)
	if err != nil {
		return decimal.Zero, fmt.Errorf("coupons received by %s: %w", investorID, err)
	}
	return db.DecimalFromNumeric(total)
}

func (r *CouponRepo) RecordPayment(ctx context.Context, q Queryer, couponID, investorID, issueID uuid.UUID, amount decimal.Decimal) error {
	if q == nil {
		q = r.pool
	}
	_, err := q.Exec(ctx, `
		INSERT INTO coupon_payments (id, coupon_id, investor_id, issue_id, amount)
		VALUES ($1,$2,$3,$4,$5)`,
		uuid.New(), couponID, investorID, issueID, db.NumericFromDecimal(amount))
	if err != nil {
		return fmt.Errorf("record coupon payment to %s: %w", investorID, err)
	}
	return nil
}

func (r *CouponRepo) UpcomingPayments(ctx context.Context, limit int) ([]*models.CouponSchedule, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, issue_id, sequence_num, payment_date, amount, status, paid_at
		FROM coupon_schedule WHERE status=$1 ORDER BY payment_date ASC LIMIT $2`,
		models.CouponScheduled, clampLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("upcoming coupons: %w", err)
	}
	defer rows.Close()
	var out []*models.CouponSchedule
	for rows.Next() {
		c, err := scanCoupon(rows)
		if err != nil {
			return nil, fmt.Errorf("scan upcoming coupon: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *CouponRepo) PaidTotals(ctx context.Context) (int64, decimal.Decimal, error) {
	var cnt int64
	var amt pgxNumeric
	err := r.pool.QueryRow(ctx, `
		SELECT count(*), COALESCE(SUM(amount),0) FROM coupon_payments`).Scan(&cnt, &amt)
	if err != nil {
		return 0, decimal.Zero, fmt.Errorf("coupon payment totals: %w", err)
	}
	d, err := db.DecimalFromNumeric(amt)
	if err != nil {
		return 0, decimal.Zero, err
	}
	return cnt, d, nil
}

func scanCoupon(row pgx.Row) (*models.CouponSchedule, error) {
	var c models.CouponSchedule
	var amt pgxNumeric
	err := row.Scan(&c.ID, &c.IssueID, &c.SequenceNum, &c.PaymentDate, &amt, &c.Status, &c.PaidAt)
	if err != nil {
		return nil, err
	}
	if c.Amount, err = db.DecimalFromNumeric(amt); err != nil {
		return nil, fmt.Errorf("decode coupon amount: %w", err)
	}
	return &c, nil
}
