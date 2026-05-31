package models

import "time"

type Direction string

const (
	DirUp   Direction = "up"
	DirDown Direction = "down"
	DirFlat Direction = "flat"
)

func (d Direction) Valid() bool {
	return d == DirUp || d == DirDown || d == DirFlat
}

type Prediction struct {
	Pair       string    `json:"pair"`
	WindowEnd  time.Time `json:"window_end"`
	Direction  Direction `json:"direction"`
	Confidence float64   `json:"confidence"`
	XGBProb    float64   `json:"xgb_prob"`
	LSTMProb   float64   `json:"lstm_prob"`
	CreatedAt  time.Time `json:"created_at"`
}
