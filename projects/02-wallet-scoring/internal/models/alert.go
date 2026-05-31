package models

import "time"

type AlertLevel string

const (
	AlertInfo     AlertLevel = "info"
	AlertWarning  AlertLevel = "warning"
	AlertCritical AlertLevel = "critical"
)

type Alert struct {
	ID        string         `json:"id"`
	Level     AlertLevel     `json:"level"`
	Wallet    string         `json:"wallet,omitempty"`
	Rule      string         `json:"rule"`
	Message   string         `json:"message"`
	Payload   map[string]any `json:"payload,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

type AlertRule struct {
	Name           string  `json:"name"`
	ScoreThreshold float32 `json:"score_threshold"`
	OnCategoryFrom string  `json:"on_category_from,omitempty"`
	OnCategoryTo   string  `json:"on_category_to,omitempty"`
}
