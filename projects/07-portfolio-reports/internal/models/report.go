package models

import "time"

type Report struct {
	Address     string    `json:"address"`
	GeneratedAt time.Time `json:"generated_at"`
	Metrics     Portfolio `json:"metrics"`
	TextReport  string    `json:"text_report"`
	Summary     string    `json:"summary"`
	Source      string    `json:"source"`
}
