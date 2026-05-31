package models

import "time"

type Portfolio struct {
	Address         string              `json:"address"`
	TotalValue      float64             `json:"total_value"`
	Tokens          []TokenPosition     `json:"tokens"`
	PnL             PnLData             `json:"pnl"`
	Diversification DiversificationData `json:"diversification"`
	Risk            RiskData            `json:"risk"`
	GeneratedAt     time.Time           `json:"generated_at"`
}
