package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

const (
	InvestorIndividual  = "individual"
	InvestorLegalEntity = "legal_entity"
)

type Investor struct {
	ID            uuid.UUID       `json:"id"`
	Name          string          `json:"name"`
	Type          string          `json:"type"`
	AccountNumber string          `json:"account_number"`
	Balance       decimal.Decimal `json:"balance"`
	CreatedAt     time.Time       `json:"created_at"`
}

func ValidInvestorType(t string) bool {
	return t == InvestorIndividual || t == InvestorLegalEntity
}
