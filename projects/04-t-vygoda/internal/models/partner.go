package models

import "time"

type Partner struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	LogoURL      *string   `json:"logo_url,omitempty"`
	CPAPercent   float64   `json:"cpa_percent"`
	ContactEmail *string   `json:"contact_email,omitempty"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type PartnerInput struct {
	Name         string  `json:"name" binding:"required,min=2,max=200"`
	LogoURL      *string `json:"logo_url,omitempty"`
	CPAPercent   float64 `json:"cpa_percent" binding:"gte=0,lte=100"`
	ContactEmail *string `json:"contact_email,omitempty"`
	Active       bool    `json:"active"`
}

type PartnerWithBalance struct {
	Partner
	Balance *CFABalance `json:"balance,omitempty"`
}
