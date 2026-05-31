package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

const (
	TradeSubmitted = "submitted"
	TradeSettled   = "settled"
	TradeFailed    = "failed"
)

type Trade struct {
	ID              uuid.UUID       `json:"id"`
	IssueID         uuid.UUID       `json:"issue_id"`
	SellerID        uuid.UUID       `json:"seller_id"`
	BuyerID         uuid.UUID       `json:"buyer_id"`
	Quantity        int64           `json:"quantity"`
	Price           decimal.Decimal `json:"price"`
	AccruedInterest decimal.Decimal `json:"accrued_interest"`
	TotalAmount     decimal.Decimal `json:"total_amount"`
	Status          string          `json:"status"`
	FailureReason   string          `json:"failure_reason,omitempty"`
	SubmittedAt     time.Time       `json:"submitted_at"`
	SettledAt       *time.Time      `json:"settled_at,omitempty"`
}
