package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Position struct {
	ID         uuid.UUID       `json:"id"`
	InvestorID uuid.UUID       `json:"investor_id"`
	IssueID    uuid.UUID       `json:"issue_id"`
	Quantity   int64           `json:"quantity"`
	AvgPrice   decimal.Decimal `json:"avg_price"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

type PositionWithIssue struct {
	Position
	IssueName    string          `json:"issue_name"`
	ISIN         string          `json:"isin"`
	Nominal      decimal.Decimal `json:"nominal"`
	LastPrice    decimal.Decimal `json:"last_price"`
	MarketValue  decimal.Decimal `json:"market_value"`
	UnrealizedPL decimal.Decimal `json:"unrealized_pl"`
	MaturityDate time.Time       `json:"maturity_date"`
	Status       string          `json:"status"`
}
