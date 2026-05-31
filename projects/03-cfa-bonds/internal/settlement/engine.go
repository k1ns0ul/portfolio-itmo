package settlement

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/andrey/cfa-bonds/internal/kafka"
	"github.com/andrey/cfa-bonds/internal/models"
	"github.com/andrey/cfa-bonds/internal/redis"
	"github.com/andrey/cfa-bonds/internal/repo"
)

type Engine struct {
	pool      *pgxpool.Pool
	issues    *repo.IssueRepo
	investors *repo.InvestorRepo
	positions *repo.PositionRepo
	trades    *repo.TradeRepo
	coupons   *repo.CouponRepo
	events    *repo.EventRepo
	cache     *redis.Client
	producer  *kafka.Producer
	log       *slog.Logger
	observe   func(time.Duration, bool)
}

type Deps struct {
	Pool      *pgxpool.Pool
	Issues    *repo.IssueRepo
	Investors *repo.InvestorRepo
	Positions *repo.PositionRepo
	Trades    *repo.TradeRepo
	Coupons   *repo.CouponRepo
	Events    *repo.EventRepo
	Cache     *redis.Client
	Producer  *kafka.Producer
	Log       *slog.Logger
	Observe   func(dur time.Duration, ok bool)
}

func NewEngine(d Deps) *Engine {
	obs := d.Observe
	if obs == nil {
		obs = func(time.Duration, bool) {}
	}
	return &Engine{
		pool:      d.Pool,
		issues:    d.Issues,
		investors: d.Investors,
		positions: d.Positions,
		trades:    d.Trades,
		coupons:   d.Coupons,
		events:    d.Events,
		cache:     d.Cache,
		producer:  d.Producer,
		log:       d.Log,
		observe:   obs,
	}
}

func (e *Engine) ProcessTrade(ctx context.Context, tradeID string) error {
	start := time.Now()
	id, err := parseUUID(tradeID)
	if err != nil {
		return fmt.Errorf("bad trade id %q: %w", tradeID, err)
	}

	trade, err := e.trades.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("load trade %s: %w", id, err)
	}
	if trade.Status != models.TradeSubmitted {
		e.log.Info("skipping non-submitted trade", "trade", id, "status", trade.Status)
		return nil
	}

	if err := e.settle(ctx, trade); err != nil {
		e.observe(time.Since(start), false)
		e.fail(ctx, trade, err)
		return err
	}
	e.observe(time.Since(start), true)
	return nil
}

func (e *Engine) settle(ctx context.Context, trade *models.Trade) error {
	issue, err := e.issues.Get(ctx, trade.IssueID)
	if err != nil {
		return fmt.Errorf("load issue: %w", err)
	}
	if issue.Status != models.IssueActive {
		return fmt.Errorf("issue %s is %s, trading requires active", issue.ID, issue.Status)
	}

	sellerPos, err := e.positions.GetByInvestorAndIssue(ctx, nil, trade.SellerID, trade.IssueID)
	if err != nil {
		return fmt.Errorf("seller has no position: %w", err)
	}
	if sellerPos.Quantity < trade.Quantity {
		return fmt.Errorf("seller holds %d, trade needs %d", sellerPos.Quantity, trade.Quantity)
	}

	lastCoupon := e.lastCouponDate(ctx, issue)
	aiPerUnit, err := CalcAccruedInterest(issue.Nominal, issue.CouponRate, issue.CouponFrequency, lastCoupon, time.Now())
	if err != nil {
		return fmt.Errorf("calc accrued interest: %w", err)
	}
	qty := decimal.NewFromInt(trade.Quantity)
	accrued := aiPerUnit.Mul(qty)
	total := trade.Price.Mul(qty).Add(accrued)

	buyer, err := e.investors.Get(ctx, trade.BuyerID)
	if err != nil {
		return fmt.Errorf("load buyer: %w", err)
	}
	if buyer.Balance.LessThan(total) {
		return fmt.Errorf("buyer balance %s below required %s", buyer.Balance, total)
	}

	tx, err := e.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return fmt.Errorf("begin settlement tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := e.positions.Transfer(ctx, tx, trade.SellerID, trade.BuyerID, trade.IssueID, trade.Quantity, trade.Price); err != nil {
		return fmt.Errorf("transfer position: %w", err)
	}
	if err := e.investors.UpdateBalanceTx(ctx, tx, trade.BuyerID, total.Neg()); err != nil {
		return fmt.Errorf("debit buyer: %w", err)
	}
	if err := e.investors.UpdateBalanceTx(ctx, tx, trade.SellerID, total); err != nil {
		return fmt.Errorf("credit seller: %w", err)
	}
	if err := e.trades.UpdateStatus(ctx, tx, trade.ID, models.TradeSettled, accrued, total, "", true); err != nil {
		return fmt.Errorf("mark trade settled: %w", err)
	}

	payload, _ := json.Marshal(map[string]any{
		"trade_id":         trade.ID,
		"issue_id":         trade.IssueID,
		"seller_id":        trade.SellerID,
		"buyer_id":         trade.BuyerID,
		"quantity":         trade.Quantity,
		"price":            trade.Price.String(),
		"accrued_interest": accrued.String(),
		"total_amount":     total.String(),
	})
	if err := e.events.Append(ctx, tx, &models.EventLog{
		EntityType: models.EntityTrade,
		EntityID:   trade.ID,
		EventType:  models.EventTradeSettled,
		Payload:    payload,
	}); err != nil {
		return fmt.Errorf("append settled event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit settlement: %w", err)
	}

	trade.Status = models.TradeSettled
	trade.AccruedInterest = accrued
	trade.TotalAmount = total

	e.afterSettle(ctx, trade)
	return nil
}

func (e *Engine) afterSettle(ctx context.Context, trade *models.Trade) {
	if e.cache != nil {
		if err := e.cache.InvalidatePortfolio(ctx, trade.BuyerID, trade.SellerID); err != nil {
			e.log.Warn("portfolio cache invalidation failed", "err", err)
		}
		if err := e.cache.SetQuote(ctx, trade.IssueID, trade.Price); err != nil {
			e.log.Warn("quote cache update failed", "err", err)
		}
	}
	if e.producer != nil {
		if err := e.producer.Publish(ctx, kafka.TopicTradeSettled, trade.ID.String(), trade); err != nil {
			e.log.Warn("publish trade.settled failed", "trade", trade.ID, "err", err)
		}
	}
	e.log.Info("trade settled", "trade", trade.ID, "total", trade.TotalAmount.String())
}

func (e *Engine) fail(ctx context.Context, trade *models.Trade, cause error) {
	reason := cause.Error()
	if err := e.trades.UpdateStatus(ctx, nil, trade.ID, models.TradeFailed, trade.AccruedInterest, trade.TotalAmount, reason, false); err != nil {
		e.log.Error("failed to mark trade failed", "trade", trade.ID, "err", err)
	}
	payload, _ := json.Marshal(map[string]any{"trade_id": trade.ID, "reason": reason})
	if err := e.events.Append(ctx, nil, &models.EventLog{
		EntityType: models.EntityTrade,
		EntityID:   trade.ID,
		EventType:  models.EventTradeFailed,
		Payload:    payload,
	}); err != nil {
		e.log.Error("failed to append fail event", "trade", trade.ID, "err", err)
	}
	if e.producer != nil {
		_ = e.producer.Publish(ctx, kafka.TopicTradeFailed, trade.ID.String(), map[string]any{
			"trade_id": trade.ID, "reason": reason,
		})
	}
	e.log.Warn("trade failed", "trade", trade.ID, "reason", reason)
}

func (e *Engine) lastCouponDate(ctx context.Context, issue *models.BondIssue) time.Time {
	coupons, err := e.coupons.GetByIssue(ctx, issue.ID)
	if err != nil {
		return issue.IssueDate
	}
	last := issue.IssueDate
	now := time.Now()
	for _, c := range coupons {
		if c.PaymentDate.After(last) && !c.PaymentDate.After(now) {
			last = c.PaymentDate
		}
	}
	return last
}
