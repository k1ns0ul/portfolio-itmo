package workers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/IBM/sarama"

	"github.com/andrey/t-vygoda/internal/config"
	"github.com/andrey/t-vygoda/internal/kafka"
	"github.com/andrey/t-vygoda/internal/models"
	"github.com/andrey/t-vygoda/internal/repo"
)

type PromoHandler struct {
	cfg       config.KafkaConfig
	promos    *repo.PromoRepo
	purchases *repo.PurchaseRepo
	partners  *repo.PartnerRepo
	producer  *kafka.Producer
}

func NewPromoHandler(cfg config.KafkaConfig, p *repo.PromoRepo, pu *repo.PurchaseRepo, pa *repo.PartnerRepo, prod *kafka.Producer) *PromoHandler {
	return &PromoHandler{cfg: cfg, promos: p, purchases: pu, partners: pa, producer: prod}
}

func (h *PromoHandler) Handle(ctx context.Context, msg *sarama.ConsumerMessage) error {
	var ev models.KafkaEvent
	if err := jsonDecode(msg.Value, &ev); err != nil {
		return fmt.Errorf("decode event: %w", err)
	}
	switch ev.Type {
	case models.EventPromoActivated:
		return h.onActivated(ctx, ev)
	case models.EventPurchaseCreated:
		return h.onPurchaseCreated(ctx, ev)
	case models.EventPurchaseConfirmed:
		return h.onPurchaseConfirmed(ctx, ev)
	}
	return nil
}

type promoActivatedPayload struct {
	PromoID   int64 `json:"promo_id"`
	UserID    int64 `json:"user_id"`
	PartnerID int64 `json:"partner_id"`
}

func (h *PromoHandler) onActivated(ctx context.Context, ev models.KafkaEvent) error {
	var p promoActivatedPayload
	if err := ev.Decode(&p); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}
	if err := h.promos.IncrementUses(ctx, p.PromoID); err != nil {
		if errors.Is(err, repo.ErrUnavailable) {
			slog.Warn("promo unavailable", "promo_id", p.PromoID)
			return nil
		}
		return fmt.Errorf("incr uses: %w", err)
	}
	slog.Info("promo activated", "promo_id", p.PromoID, "user_id", p.UserID)
	return nil
}

func (h *PromoHandler) onPurchaseCreated(_ context.Context, ev models.KafkaEvent) error {
	var purchase models.Purchase
	if err := ev.Decode(&purchase); err != nil {
		return fmt.Errorf("decode purchase: %w", err)
	}
	slog.Info("purchase pending", "purchase_id", purchase.ID, "user_id", purchase.UserID)
	return nil
}

type purchaseConfirmPayload struct {
	PurchaseID int64   `json:"purchase_id"`
	UserID     int64   `json:"user_id"`
	PromoID    int64   `json:"promo_id"`
	PartnerID  int64   `json:"partner_id"`
	Amount     float64 `json:"amount"`
}

func (h *PromoHandler) onPurchaseConfirmed(ctx context.Context, ev models.KafkaEvent) error {
	var p purchaseConfirmPayload
	if err := ev.Decode(&p); err != nil {
		return fmt.Errorf("decode confirm: %w", err)
	}
	partner, err := h.partners.GetByID(ctx, p.PartnerID)
	if err != nil {
		return fmt.Errorf("get partner: %w", err)
	}
	cpa := p.Amount * partner.CPAPercent / 100.0

	confirmed, err := h.purchases.Confirm(ctx, p.PurchaseID, cpa)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			slog.Warn("purchase already confirmed or missing", "purchase_id", p.PurchaseID)
			return nil
		}
		return fmt.Errorf("confirm purchase: %w", err)
	}

	out, err := models.NewEvent(models.EventPurchaseConfirmed, "promo-worker", confirmed)
	if err != nil {
		return fmt.Errorf("build event: %w", err)
	}
	if err := h.producer.Publish(h.cfg.TopicPurchases, strconv.FormatInt(confirmed.ID, 10), out); err != nil {
		return fmt.Errorf("publish confirmed: %w", err)
	}
	slog.Info("purchase confirmed", "purchase_id", confirmed.ID, "cpa", cpa)
	return nil
}
