package models

import "time"

type ReferralLevel int16

const (
	RefLevel1 ReferralLevel = 1
	RefLevel2 ReferralLevel = 2
	RefLevel3 ReferralLevel = 3
)

func (l ReferralLevel) Share() float64 {
	switch l {
	case RefLevel1:
		return 0.10
	case RefLevel2:
		return 0.03
	case RefLevel3:
		return 0.01
	}
	return 0
}

type ReferralChain struct {
	ID         int64         `json:"id"`
	UserID     int64         `json:"user_id"`
	ReferrerID int64         `json:"referrer_id"`
	Level      ReferralLevel `json:"level"`
	CreatedAt  time.Time     `json:"created_at"`
}

type ReferralBonus struct {
	ID         int64         `json:"id"`
	ChainID    int64         `json:"chain_id"`
	PurchaseID int64         `json:"purchase_id"`
	ReferrerID int64         `json:"referrer_id"`
	Amount     float64       `json:"amount"`
	Level      ReferralLevel `json:"level"`
	Status     string        `json:"status"`
	CreatedAt  time.Time     `json:"created_at"`
}

type ReferralTreeNode struct {
	ReferrerID int64         `json:"referrer_id"`
	Level      ReferralLevel `json:"level"`
	ChainID    int64         `json:"chain_id"`
}
