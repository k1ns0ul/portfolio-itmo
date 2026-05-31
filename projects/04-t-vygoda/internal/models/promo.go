package models

import "time"

type PromoType string

const (
	PromoTypePercent PromoType = "percent"
	PromoTypeFixed   PromoType = "fixed"
)

func (t PromoType) Valid() bool {
	return t == PromoTypePercent || t == PromoTypeFixed
}

type Promo struct {
	ID          int64     `json:"id"`
	PartnerID   int64     `json:"partner_id"`
	Code        string    `json:"code"`
	Discount    float64   `json:"discount"`
	Type        PromoType `json:"type"`
	CategoryID  *int64    `json:"category_id,omitempty"`
	MaxUses     int       `json:"max_uses"`
	CurrentUses int       `json:"current_uses"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
}

func (p Promo) Available() bool {
	if !p.Active {
		return false
	}
	if p.MaxUses > 0 && p.CurrentUses >= p.MaxUses {
		return false
	}
	if p.ExpiresAt != nil && time.Now().After(*p.ExpiresAt) {
		return false
	}
	return true
}

type CreatePromoInput struct {
	PartnerID  int64     `json:"partner_id" binding:"required"`
	Code       string    `json:"code" binding:"required,min=3,max=50"`
	Discount   float64   `json:"discount" binding:"required,gt=0"`
	Type       PromoType `json:"type" binding:"required"`
	CategoryID *int64    `json:"category_id,omitempty"`
	MaxUses    int       `json:"max_uses"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}
