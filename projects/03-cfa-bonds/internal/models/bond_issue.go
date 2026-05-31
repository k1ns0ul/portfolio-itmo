package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

const (
	IssueDraft     = "draft"
	IssuePlacement = "placement"
	IssueActive    = "active"
	IssueMatured   = "matured"
	IssueCancelled = "cancelled"
)

type BondIssue struct {
	ID              uuid.UUID       `json:"id"`
	IssuerID        uuid.UUID       `json:"issuer_id"`
	Name            string          `json:"name"`
	ISIN            string          `json:"isin"`
	Nominal         decimal.Decimal `json:"nominal"`
	CouponRate      decimal.Decimal `json:"coupon_rate"`
	CouponFrequency int             `json:"coupon_frequency"`
	IssueDate       time.Time       `json:"issue_date"`
	MaturityDate    time.Time       `json:"maturity_date"`
	TotalQuantity   int64           `json:"total_quantity"`
	PlacedQuantity  int64           `json:"placed_quantity"`
	Status          string          `json:"status"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

var allowedTransitions = map[string][]string{
	IssueDraft:     {IssuePlacement, IssueCancelled},
	IssuePlacement: {IssueActive, IssueCancelled},
	IssueActive:    {IssueMatured},
	IssueMatured:   {},
	IssueCancelled: {},
}

func CanTransition(from, to string) bool {
	for _, t := range allowedTransitions[from] {
		if t == to {
			return true
		}
	}
	return false
}

func ValidCouponFrequency(f int) bool {
	return f == 1 || f == 2 || f == 4
}
