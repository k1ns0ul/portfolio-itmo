package coupon

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/andrey/cfa-bonds/internal/kafka"
	"github.com/andrey/cfa-bonds/internal/models"
	"github.com/andrey/cfa-bonds/internal/redis"
	"github.com/andrey/cfa-bonds/internal/repo"
)

type Service struct {
	pool      *pgxpool.Pool
	issues    *repo.IssueRepo
	investors *repo.InvestorRepo
	positions *repo.PositionRepo
	coupons   *repo.CouponRepo
	events    *repo.EventRepo
	cache     *redis.Client
	producer  *kafka.Producer
	log       *slog.Logger
}

type Deps struct {
	Pool      *pgxpool.Pool
	Issues    *repo.IssueRepo
	Investors *repo.InvestorRepo
	Positions *repo.PositionRepo
	Coupons   *repo.CouponRepo
	Events    *repo.EventRepo
	Cache     *redis.Client
	Producer  *kafka.Producer
	Log       *slog.Logger
}

func NewService(d Deps) *Service {
	return &Service{
		pool:      d.Pool,
		issues:    d.Issues,
		investors: d.Investors,
		positions: d.Positions,
		coupons:   d.Coupons,
		events:    d.Events,
		cache:     d.Cache,
		producer:  d.Producer,
		log:       d.Log,
	}
}

func (s *Service) ProcessDueCoupons(ctx context.Context, asOf time.Time) error {
	issues, err := s.issues.GetActiveWithCouponsDue(ctx, asOf)
	if err != nil {
		return fmt.Errorf("find issues with due coupons: %w", err)
	}
	s.log.Info("coupon run starting", "as_of", asOf.Format("2006-01-02"), "issues", len(issues))

	var firstErr error
	processed := 0
	for _, issue := range issues {
		if err := s.processIssue(ctx, issue); err != nil {
			s.log.Error("coupon processing failed for issue", "issue", issue.ID, "err", err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		processed++
	}
	s.log.Info("coupon run finished", "processed", processed, "total", len(issues))
	return firstErr
}

func (s *Service) processIssue(ctx context.Context, issue *models.BondIssue) error {
	coupon, err := s.coupons.GetNextDue(ctx, nil, issue.ID)
	if err != nil {
		return fmt.Errorf("next coupon for %s: %w", issue.ID, err)
	}
	holders, err := s.positions.GetByIssue(ctx, nil, issue.ID)
	if err != nil {
		return fmt.Errorf("holders of %s: %w", issue.ID, err)
	}

	perUnit := issue.Nominal.Mul(issue.CouponRate).Div(decimal.NewFromInt(int64(issue.CouponFrequency))).Round(6)

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return fmt.Errorf("begin coupon tx: %w", err)
	}
	defer tx.Rollback(ctx)

	paid := decimal.Zero
	creditedTo := make([]uuid.UUID, 0, len(holders))
	for _, h := range holders {
		amount := perUnit.Mul(decimal.NewFromInt(h.Quantity))
		if amount.LessThanOrEqual(decimal.Zero) {
			continue
		}
		if err := s.investors.UpdateBalanceTx(ctx, tx, h.InvestorID, amount); err != nil {
			return fmt.Errorf("credit coupon to %s: %w", h.InvestorID, err)
		}
		if err := s.coupons.RecordPayment(ctx, tx, coupon.ID, h.InvestorID, issue.ID, amount); err != nil {
			return fmt.Errorf("record coupon payment: %w", err)
		}
		payload, _ := json.Marshal(map[string]any{
			"issue_id":    issue.ID,
			"coupon_id":   coupon.ID,
			"investor_id": h.InvestorID,
			"quantity":    h.Quantity,
			"amount":      amount.String(),
			"sequence":    coupon.SequenceNum,
		})
		if err := s.events.Append(ctx, tx, &models.EventLog{
			EntityType: models.EntityInvestor,
			EntityID:   h.InvestorID,
			EventType:  models.EventCouponPaid,
			Payload:    payload,
		}); err != nil {
			return fmt.Errorf("append coupon event: %w", err)
		}
		paid = paid.Add(amount)
		creditedTo = append(creditedTo, h.InvestorID)
	}

	if err := s.coupons.MarkPaid(ctx, tx, coupon.ID); err != nil {
		return fmt.Errorf("finalize coupon %s: %w", coupon.ID, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit coupon payout: %w", err)
	}

	if s.cache != nil && len(creditedTo) > 0 {
		if err := s.cache.InvalidatePortfolio(ctx, creditedTo...); err != nil {
			s.log.Warn("invalidate portfolios after coupon", "err", err)
		}
	}
	if s.producer != nil {
		_ = s.producer.Publish(ctx, kafka.TopicCouponPaid, issue.ID.String(), map[string]any{
			"issue_id":     issue.ID,
			"coupon_id":    coupon.ID,
			"sequence":     coupon.SequenceNum,
			"holders":      len(creditedTo),
			"total_paid":   paid.String(),
			"payment_date": coupon.PaymentDate,
		})
	}
	s.log.Info("coupon paid", "issue", issue.ID, "sequence", coupon.SequenceNum, "holders", len(creditedTo), "total", paid.String())
	return nil
}
