package models

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type PortfolioSummary struct {
	InvestorID      uuid.UUID           `json:"investor_id"`
	Cash            decimal.Decimal     `json:"cash"`
	Positions       []PositionWithIssue `json:"positions"`
	TotalValue      decimal.Decimal     `json:"total_value"`
	TotalPnL        decimal.Decimal     `json:"total_pnl"`
	CouponsReceived decimal.Decimal     `json:"coupons_received"`
}
