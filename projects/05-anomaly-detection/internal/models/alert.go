package models

import "time"

type AlertLevel string

const (
	LevelInfo     AlertLevel = "info"
	LevelWarning  AlertLevel = "warning"
	LevelCritical AlertLevel = "critical"
)

type Alert struct {
	ID               string     `json:"id"`
	TxID             string     `json:"tx_id"`
	ClientID         string     `json:"client_id"`
	Score            float64    `json:"score"`
	IforestFlag      bool       `json:"iforest_flag"`
	AutoencoderScore float64    `json:"autoencoder_score"`
	Level            AlertLevel `json:"level"`
	CreatedAt        time.Time  `json:"created_at"`
}
