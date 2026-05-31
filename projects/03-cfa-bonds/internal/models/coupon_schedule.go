package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

const (
	CouponScheduled  = "scheduled"
	CouponProcessing = "processing"
	CouponPaid       = "paid"
	CouponFailed     = "failed"
)

type CouponSchedule struct {
	ID          uuid.UUID       `json:"id"`
	IssueID     uuid.UUID       `json:"issue_id"`
	SequenceNum int             `json:"sequence_num"`
	PaymentDate time.Time       `json:"payment_date"`
	Amount      decimal.Decimal `json:"amount"`
	Status      string          `json:"status"`
	PaidAt      *time.Time      `json:"paid_at,omitempty"`
}
