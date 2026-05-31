package models

import "time"

type Transaction struct {
	ID             string    `json:"id"`
	ClientID       string    `json:"client_id"`
	Amount         float64   `json:"amount"`
	CounterpartyID string    `json:"counterparty_id"`
	Timestamp      time.Time `json:"ts"`
	Category       string    `json:"category"`
	Channel        string    `json:"channel"`
}
