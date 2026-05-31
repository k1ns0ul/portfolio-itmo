package maturity

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
		events:    d.Events,
		cache:     d.Cache,
		producer:  d.Producer,
		log:       d.Log,
	}
}

func (s *Service) ProcessMaturities(ctx context.Context, asOf time.Time) error {
	issues, err := s.issues.GetMaturingIssues(ctx, asOf)
	if err != nil {
		return fmt.Errorf("find maturing issues: %w", err)
	}
	s.log.Info("maturity run starting", "as_of", asOf.Format("2006-01-02"), "issues", len(issues))

	var firstErr error
	for _, issue := range issues {
		if err := s.redeemIssue(ctx, issue); err != nil {
			s.log.Error("redemption failed", "issue", issue.ID, "err", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (s *Service) redeemIssue(ctx context.Context, issue *models.BondIssue) error {
	holders, err := s.positions.GetByIssue(ctx, nil, issue.ID)
	if err != nil {
		return fmt.Errorf("holders of maturing issue %s: %w", issue.ID, err)
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return fmt.Errorf("begin maturity tx: %w", err)
	}
	defer tx.Rollback(ctx)

	redeemed := decimal.Zero
	affected := make([]uuid.UUID, 0, len(holders))
	for _, h := range holders {
		amount := issue.Nominal.Mul(decimal.NewFromInt(h.Quantity))
		if err := s.investors.UpdateBalanceTx(ctx, tx, h.InvestorID, amount); err != nil {
			return fmt.Errorf("redeem nominal to %s: %w", h.InvestorID, err)
		}
		payload, _ := json.Marshal(map[string]any{
			"issue_id":    issue.ID,
			"investor_id": h.InvestorID,
			"quantity":    h.Quantity,
			"redeemed":    amount.String(),
		})
		if err := s.events.Append(ctx, tx, &models.EventLog{
			EntityType: models.EntityInvestor,
			EntityID:   h.InvestorID,
			EventType:  models.EventIssueMatured,
			Payload:    payload,
		}); err != nil {
			return fmt.Errorf("append maturity event: %w", err)
		}
		redeemed = redeemed.Add(amount)
		affected = append(affected, h.InvestorID)
	}

	if err := s.positions.ZeroOutByIssue(ctx, tx, issue.ID); err != nil {
		return fmt.Errorf("clear positions: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE bond_issues SET status=$1, updated_at=now() WHERE id=$2 AND status=$3`,
		models.IssueMatured, issue.ID, models.IssueActive); err != nil {
		return fmt.Errorf("close issue %s: %w", issue.ID, err)
	}

	issuePayload, _ := json.Marshal(map[string]any{
		"issue_id":       issue.ID,
		"holders":        len(affected),
		"total_redeemed": redeemed.String(),
		"maturity_date":  issue.MaturityDate,
	})
	if err := s.events.Append(ctx, tx, &models.EventLog{
		EntityType: models.EntityIssue,
		EntityID:   issue.ID,
		EventType:  models.EventIssueMatured,
		Payload:    issuePayload,
	}); err != nil {
		return fmt.Errorf("append issue maturity event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit redemption: %w", err)
	}

	if s.cache != nil && len(affected) > 0 {
		if err := s.cache.InvalidatePortfolio(ctx, affected...); err != nil {
			s.log.Warn("invalidate portfolios after maturity", "err", err)
		}
	}
	if s.producer != nil {
		_ = s.producer.Publish(ctx, kafka.TopicIssueMatured, issue.ID.String(), map[string]any{
			"issue_id":       issue.ID,
			"holders":        len(affected),
			"total_redeemed": redeemed.String(),
		})
	}
	s.log.Info("issue matured", "issue", issue.ID, "holders", len(affected), "redeemed", redeemed.String())
	return nil
}
