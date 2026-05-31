package models

import "time"

type DebtorType string

const (
	DebtorBank    DebtorType = "bank"
	DebtorPartner DebtorType = "partner"
)

func (t DebtorType) Valid() bool {
	return t == DebtorBank || t == DebtorPartner
}

type CFAStatus string

const (
	CFACreated   CFAStatus = "created"
	CFAConfirmed CFAStatus = "confirmed"
	CFASettled   CFAStatus = "settled"
)

func (s CFAStatus) Valid() bool {
	switch s {
	case CFACreated, CFAConfirmed, CFASettled:
		return true
	}
	return false
}

type CFASettlement struct {
	ID         int64      `json:"id"`
	PurchaseID int64      `json:"purchase_id"`
	PartnerID  int64      `json:"partner_id"`
	DebtorType DebtorType `json:"debtor_type"`
	Amount     float64    `json:"amount"`
	Status     CFAStatus  `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	SettledAt  *time.Time `json:"settled_at,omitempty"`
}

type CFABalance struct {
	PartnerID   int64     `json:"partner_id"`
	BankOwes    float64   `json:"bank_owes"`
	PartnerOwes float64   `json:"partner_owes"`
	NetBalance  float64   `json:"net_balance"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CFAReconciliation struct {
	ID            int64     `json:"id"`
	PartnerID     int64     `json:"partner_id"`
	SettledAmount float64   `json:"settled_amount"`
	SettledAt     time.Time `json:"settled_at"`
}
