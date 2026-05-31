package models

import "time"

type Category string

const (
	CategoryLegit      Category = "legit"
	CategorySuspicious Category = "suspicious"
	CategoryScam       Category = "scam"
	CategoryUnknown    Category = "unknown"
)

func (c Category) Valid() bool {
	switch c {
	case CategoryLegit, CategorySuspicious, CategoryScam, CategoryUnknown:
		return true
	}
	return false
}

type WalletStats struct {
	Wallet               string    `json:"wallet"`
	TxCount              uint64    `json:"tx_count"`
	FirstSeen            time.Time `json:"first_seen"`
	LastSeen             time.Time `json:"last_seen"`
	UniqueCounterparties uint32    `json:"unique_counterparties"`
	AvgTxAmount          float64   `json:"avg_tx_amount"`
	MedianTxAmount       float64   `json:"median_tx_amount"`
	HerfindahlIndex      float64   `json:"herfindahl_index"`
	SmartContractRatio   float32   `json:"smart_contract_ratio"`
	VelocityPerHour      float64   `json:"velocity_per_hour"`
	DormancyDays         float64   `json:"dormancy_days"`
	Score                float32   `json:"score"`
	Category             Category  `json:"category"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type WalletScore struct {
	Wallet    string    `json:"wallet"`
	Score     float32   `json:"score"`
	Previous  float32   `json:"previous_score"`
	Category  Category  `json:"category"`
	Reason    string    `json:"reason,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type TokenScore struct {
	Mint       string    `json:"mint"`
	Category   Category  `json:"category"`
	Confidence float32   `json:"confidence"`
	RiskScore  float32   `json:"risk_score"`
	Holders    uint32    `json:"holders"`
	Volume24h  float64   `json:"volume_24h"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type ScoreDistribution struct {
	Buckets []ScoreBucket `json:"buckets"`
}

type ScoreBucket struct {
	From  float32 `json:"from"`
	To    float32 `json:"to"`
	Count uint64  `json:"count"`
}
