package models

import "time"

type TransactionFeatures struct {
	TxID                    string    `json:"tx_id"`
	ClientID                string    `json:"client_id"`
	Amount                  float64   `json:"amount"`
	AvgAmount1h             float64   `json:"avg_amount_1h"`
	AvgAmount24h            float64   `json:"avg_amount_24h"`
	UniqueCounterparties24h float64   `json:"unique_counterparties_24h"`
	ZScore                  float64   `json:"z_score"`
	TimeSinceLastTx         float64   `json:"time_since_last_tx"`
	NightFlag               float64   `json:"night_flag"`
	FrequencyScore          float64   `json:"frequency_score"`
	Timestamp               time.Time `json:"ts"`
}

func (f TransactionFeatures) Vector() []float64 {
	return []float64{
		f.Amount, f.AvgAmount1h, f.AvgAmount24h,
		f.UniqueCounterparties24h, f.ZScore,
		f.TimeSinceLastTx, f.NightFlag, f.FrequencyScore,
	}
}
