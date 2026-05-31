package notifier

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/andrey/wallet-scoring/internal/models"
)

type Rule func(prev, next models.WalletScore) []models.Alert

type Config struct {
	ScoreThreshold float32
}

func DefaultRules(cfg Config) []Rule {
	return []Rule{
		dropBelowThreshold(cfg.ScoreThreshold),
		categoryFlip(),
		sharpDrop(20),
	}
}

func dropBelowThreshold(th float32) Rule {
	return func(prev, next models.WalletScore) []models.Alert {
		if next.Score >= th {
			return nil
		}
		if prev.Score != 0 && prev.Score < th {
			return nil
		}
		return []models.Alert{newAlert(models.AlertWarning, next.Wallet, "score_below_threshold",
			fmt.Sprintf("score dropped to %.1f (threshold %.1f)", next.Score, th),
			map[string]any{"score": next.Score, "threshold": th})}
	}
}

func categoryFlip() Rule {
	return func(prev, next models.WalletScore) []models.Alert {
		if prev.Category == "" || prev.Category == next.Category {
			return nil
		}
		level := models.AlertInfo
		if next.Category == models.CategoryScam {
			level = models.AlertCritical
		} else if next.Category == models.CategorySuspicious {
			level = models.AlertWarning
		}
		return []models.Alert{newAlert(level, next.Wallet, "category_flip",
			fmt.Sprintf("category %s -> %s", prev.Category, next.Category),
			map[string]any{"from": prev.Category, "to": next.Category})}
	}
}

func sharpDrop(delta float32) Rule {
	return func(prev, next models.WalletScore) []models.Alert {
		if prev.Score == 0 {
			return nil
		}
		if prev.Score-next.Score < delta {
			return nil
		}
		return []models.Alert{newAlert(models.AlertWarning, next.Wallet, "sharp_drop",
			fmt.Sprintf("score dropped by %.1f (from %.1f to %.1f)", prev.Score-next.Score, prev.Score, next.Score),
			map[string]any{"delta": prev.Score - next.Score, "prev": prev.Score, "next": next.Score})}
	}
}

func newAlert(level models.AlertLevel, wallet, rule, msg string, payload map[string]any) models.Alert {
	return models.Alert{
		ID:        randID(),
		Level:     level,
		Wallet:    wallet,
		Rule:      rule,
		Message:   msg,
		Payload:   payload,
		CreatedAt: time.Now().UTC(),
	}
}

func randID() string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
