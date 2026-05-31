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

type PositionRepo struct {
	pool *pgxpool.Pool
}

func NewPositionRepo(pool *pgxpool.Pool) *PositionRepo {
	return &PositionRepo{pool: pool}
}

func (r *PositionRepo) GetByInvestorAndIssue(ctx context.Context, q Queryer, investorID, issueID uuid.UUID) (*models.Position, error) {
	if q == nil {
		q = r.pool
	}
	row := q.QueryRow(ctx, `
		SELECT id, investor_id, issue_id, quantity, avg_price, updated_at
		FROM positions WHERE investor_id=$1 AND issue_id=$2`, investorID, issueID)
	pos, err := scanPosition(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get position %s/%s: %w", investorID, issueID, err)
	}
	return pos, nil
}

func (r *PositionRepo) Upsert(ctx context.Context, q Queryer, pos *models.Position) error {
	if q == nil {
		q = r.pool
	}
	if pos.ID == uuid.Nil {
		pos.ID = uuid.New()
	}
	_, err := q.Exec(ctx, `
		INSERT INTO positions (id, investor_id, issue_id, quantity, avg_price, updated_at)
		VALUES ($1,$2,$3,$4,$5, now())
		ON CONFLICT (investor_id, issue_id)
		DO UPDATE SET quantity = EXCLUDED.quantity, avg_price = EXCLUDED.avg_price, updated_at = now()`,
		pos.ID, pos.InvestorID, pos.IssueID, pos.Quantity, db.NumericFromDecimal(pos.AvgPrice))
	if err != nil {
		return fmt.Errorf("upsert position %s/%s: %w", pos.InvestorID, pos.IssueID, err)
	}
	return nil
}

func (r *PositionRepo) GetByInvestor(ctx context.Context, investorID uuid.UUID) ([]*models.PositionWithIssue, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.id, p.investor_id, p.issue_id, p.quantity, p.avg_price, p.updated_at,
		       bi.name, bi.isin, bi.nominal, bi.maturity_date, bi.status,
		       COALESCE(q.last_price, bi.nominal) AS last_price
		FROM positions p
		JOIN bond_issues bi ON bi.id = p.issue_id
		LEFT JOIN LATERAL (
			SELECT price AS last_price FROM trades t
			WHERE t.issue_id = p.issue_id AND t.status = 'settled'
			ORDER BY t.settled_at DESC NULLS LAST LIMIT 1
		) q ON true
		WHERE p.investor_id = $1 AND p.quantity > 0
		ORDER BY bi.name`, investorID)
	if err != nil {
		return nil, fmt.Errorf("positions for investor %s: %w", investorID, err)
	}
	defer rows.Close()
	return collectPositionsWithIssue(rows)
}

func (r *PositionRepo) GetByIssue(ctx context.Context, q Queryer, issueID uuid.UUID) ([]*models.Position, error) {
	if q == nil {
		q = r.pool
	}
	rows, err := q.Query(ctx, `
		SELECT id, investor_id, issue_id, quantity, avg_price, updated_at
		FROM positions WHERE issue_id=$1 AND quantity > 0 ORDER BY quantity DESC`, issueID)
	if err != nil {
		return nil, fmt.Errorf("holders of issue %s: %w", issueID, err)
	}
	defer rows.Close()
	var out []*models.Position
	for rows.Next() {
		pos, err := scanPosition(rows)
		if err != nil {
			return nil, fmt.Errorf("scan holder: %w", err)
		}
		out = append(out, pos)
	}
	return out, rows.Err()
}

func (r *PositionRepo) Transfer(ctx context.Context, tx pgx.Tx, sellerID, buyerID, issueID uuid.UUID, quantity int64, price decimal.Decimal) error {
	seller, err := r.GetByInvestorAndIssue(ctx, tx, sellerID, issueID)
	if err != nil {
		return fmt.Errorf("load seller position: %w", err)
	}
	if seller.Quantity < quantity {
		return fmt.Errorf("seller %s holds %d but trade needs %d: %w", sellerID, seller.Quantity, quantity, ErrConflict)
	}
	seller.Quantity -= quantity
	if err := r.Upsert(ctx, tx, seller); err != nil {
		return fmt.Errorf("debit seller: %w", err)
	}

	buyer, err := r.GetByInvestorAndIssue(ctx, tx, buyerID, issueID)
	if errors.Is(err, ErrNotFound) {
		buyer = &models.Position{InvestorID: buyerID, IssueID: issueID, Quantity: 0, AvgPrice: decimal.Zero}
	} else if err != nil {
		return fmt.Errorf("load buyer position: %w", err)
	}

	newQty := buyer.Quantity + quantity
	prevCost := buyer.AvgPrice.Mul(decimal.NewFromInt(buyer.Quantity))
	addCost := price.Mul(decimal.NewFromInt(quantity))
	buyer.AvgPrice = prevCost.Add(addCost).Div(decimal.NewFromInt(newQty)).Round(8)
	buyer.Quantity = newQty
	if err := r.Upsert(ctx, tx, buyer); err != nil {
		return fmt.Errorf("credit buyer: %w", err)
	}
	return nil
}

func (r *PositionRepo) ZeroOutByIssue(ctx context.Context, q Queryer, issueID uuid.UUID) error {
	if q == nil {
		q = r.pool
	}
	_, err := q.Exec(ctx, `UPDATE positions SET quantity = 0, updated_at = now() WHERE issue_id=$1`, issueID)
	if err != nil {
		return fmt.Errorf("zero positions for issue %s: %w", issueID, err)
	}
	return nil
}

func scanPosition(row pgx.Row) (*models.Position, error) {
	var p models.Position
	var avg pgxNumeric
	err := row.Scan(&p.ID, &p.InvestorID, &p.IssueID, &p.Quantity, &avg, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if p.AvgPrice, err = db.DecimalFromNumeric(avg); err != nil {
		return nil, fmt.Errorf("decode avg_price: %w", err)
	}
	return &p, nil
}

func collectPositionsWithIssue(rows pgx.Rows) ([]*models.PositionWithIssue, error) {
	var out []*models.PositionWithIssue
	for rows.Next() {
		var pw models.PositionWithIssue
		var avg, nominal, last pgxNumeric
		err := rows.Scan(&pw.ID, &pw.InvestorID, &pw.IssueID, &pw.Quantity, &avg, &pw.UpdatedAt,
			&pw.IssueName, &pw.ISIN, &nominal, &pw.MaturityDate, &pw.Status, &last)
		if err != nil {
			return nil, fmt.Errorf("scan position row: %w", err)
		}
		if pw.AvgPrice, err = db.DecimalFromNumeric(avg); err != nil {
			return nil, err
		}
		if pw.Nominal, err = db.DecimalFromNumeric(nominal); err != nil {
			return nil, err
		}
		if pw.LastPrice, err = db.DecimalFromNumeric(last); err != nil {
			return nil, err
		}
		qty := decimal.NewFromInt(pw.Quantity)
		pw.MarketValue = pw.LastPrice.Mul(qty)
		pw.UnrealizedPL = pw.LastPrice.Sub(pw.AvgPrice).Mul(qty)
		out = append(out, &pw)
	}
	return out, rows.Err()
}
