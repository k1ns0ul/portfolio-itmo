package workers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/IBM/sarama"

	"github.com/andrey/t-vygoda/internal/config"
	"github.com/andrey/t-vygoda/internal/kafka"
	"github.com/andrey/t-vygoda/internal/models"
	"github.com/andrey/t-vygoda/internal/repo"
)

type CFAHandler struct {
	cfg        config.KafkaConfig
	worker     config.WorkerConfig
	cfa        *repo.CFARepo
	producer   *kafka.Producer
}

func NewCFAHandler(cfg config.KafkaConfig, w config.WorkerConfig, cfa *repo.CFARepo, prod *kafka.Producer) *CFAHandler {
	return &CFAHandler{cfg: cfg, worker: w, cfa: cfa, producer: prod}
}

func (h *CFAHandler) Handle(ctx context.Context, msg *sarama.ConsumerMessage) error {
	var ev models.KafkaEvent
	if err := jsonDecode(msg.Value, &ev); err != nil {
		return fmt.Errorf("decode event: %w", err)
	}
	if ev.Type != models.EventPurchaseConfirmed {
		return nil
	}
	var p models.Purchase
	if err := ev.Decode(&p); err != nil {
		return fmt.Errorf("decode purchase: %w", err)
	}
	if p.Status != models.PurchaseConfirmed || p.CPAAmount <= 0 {
		return nil
	}

	settlement, err := h.cfa.CreateSettlement(ctx, repo.CreateSettlementInput{
		PurchaseID: p.ID,
		PartnerID:  p.PartnerID,
		DebtorType: models.DebtorBank,
		Amount:     p.CPAAmount,
	})
	if err != nil {
		if errors.Is(err, repo.ErrDuplicate) {
			slog.Debug("settlement already exists", "purchase_id", p.ID)
			return nil
		}
		return fmt.Errorf("create settlement: %w", err)
	}

	out, err := models.NewEvent(models.EventCFASettlement, "cfa-worker", settlement)
	if err != nil {
		return fmt.Errorf("build event: %w", err)
	}
	if err := h.producer.Publish(h.cfg.TopicCFA, strconv.FormatInt(settlement.PartnerID, 10), out); err != nil {
		return fmt.Errorf("publish settlement: %w", err)
	}
	slog.Info("cfa settlement",
		"id", settlement.ID,
		"partner_id", settlement.PartnerID,
		"amount", settlement.Amount,
	)
	return nil
}

func (h *CFAHandler) RunReconcileLoop(ctx context.Context) {
	t := time.NewTicker(h.worker.CFAReconcileInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			h.scanCandidates(ctx)
		}
	}
}

func (h *CFAHandler) scanCandidates(ctx context.Context) {
	balances, err := h.cfa.ListBalances(ctx)
	if err != nil {
		slog.Error("list balances", "err", err)
		return
	}
	for _, b := range balances {
		if abs(b.NetBalance) >= h.worker.CFANetThreshold {
			slog.Info("settlement candidate",
				"partner_id", b.PartnerID,
				"net_balance", b.NetBalance,
				"bank_owes", b.BankOwes,
				"partner_owes", b.PartnerOwes,
			)
		}
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
