package models

import "time"

type PurchaseStatus string

const (
	PurchasePending   PurchaseStatus = "pending"
	PurchaseConfirmed PurchaseStatus = "confirmed"
	PurchaseCancelled PurchaseStatus = "cancelled"
)

func (s PurchaseStatus) Valid() bool {
	switch s {
	case PurchasePending, PurchaseConfirmed, PurchaseCancelled:
		return true
	}
	return false
}

type Purchase struct {
	ID          int64          `json:"id"`
	UserID      int64          `json:"user_id"`
	PromoID     int64          `json:"promo_id"`
	PartnerID   int64          `json:"partner_id"`
	Amount      float64        `json:"amount"`
	CPAAmount   float64        `json:"cpa_amount"`
	Status      PurchaseStatus `json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	ConfirmedAt *time.Time     `json:"confirmed_at,omitempty"`
}

type CreatePurchaseInput struct {
	PromoID int64   `json:"promo_id" binding:"required"`
	Amount  float64 `json:"amount" binding:"required,gt=0"`
}
