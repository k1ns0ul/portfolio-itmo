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
	rds "github.com/andrey/t-vygoda/internal/redis"
	"github.com/andrey/t-vygoda/internal/repo"
)

type ReferralHandler struct {
	cfg         config.KafkaConfig
	purchases   *repo.PurchaseRepo
	referrals   *repo.ReferralRepo
	leaderboard *rds.Leaderboard
	producer    *kafka.Producer
}

func NewReferralHandler(cfg config.KafkaConfig, purchases *repo.PurchaseRepo, refs *repo.ReferralRepo, lb *rds.Leaderboard, prod *kafka.Producer) *ReferralHandler {
	return &ReferralHandler{cfg: cfg, purchases: purchases, referrals: refs, leaderboard: lb, producer: prod}
}

func (h *ReferralHandler) Handle(ctx context.Context, msg *sarama.ConsumerMessage) error {
	var ev models.KafkaEvent
	if err := jsonDecode(msg.Value, &ev); err != nil {
		return fmt.Errorf("decode event: %w", err)
	}
	if ev.Type != models.EventPurchaseConfirmed {
		return nil
	}
	var purchase models.Purchase
	if err := ev.Decode(&purchase); err != nil {
		return fmt.Errorf("decode purchase: %w", err)
	}
	if purchase.Status != models.PurchaseConfirmed {
		return nil
	}

	tree, err := h.referrals.GetTreeUpTo3Levels(ctx, purchase.UserID)
	if err != nil {
		return fmt.Errorf("ref tree: %w", err)
	}
	if len(tree) == 0 {
		return nil
	}

	for _, node := range tree {
		amount := purchase.CPAAmount * node.Level.Share()
		if amount <= 0 {
			continue
		}
		bonus, err := h.referrals.CreateBonus(ctx, repo.CreateBonusInput{
			ChainID:    node.ChainID,
			PurchaseID: purchase.ID,
			ReferrerID: node.ReferrerID,
			Amount:     amount,
			Level:      node.Level,
		})
		if err != nil {
			if errors.Is(err, repo.ErrDuplicate) {
				continue
			}
			return fmt.Errorf("create bonus: %w", err)
		}

		if node.Level == models.RefLevel1 {
			if err := h.leaderboard.IncrBy(ctx, rds.LeaderboardReferrals, node.ReferrerID, 1); err != nil {
				slog.Warn("leaderboard incr", "err", err, "referrer", node.ReferrerID)
			}
		}

		out, err := models.NewEvent(models.EventReferralCredited, "referral-worker", bonus)
		if err != nil {
			return fmt.Errorf("build event: %w", err)
		}
		if err := h.producer.Publish(h.cfg.TopicReferrals, strconv.FormatInt(node.ReferrerID, 10), out); err != nil {
			return fmt.Errorf("publish credited: %w", err)
		}
		slog.Info("bonus credited",
			"referrer_id", node.ReferrerID, "level", node.Level, "amount", amount)
	}
	return nil
}
