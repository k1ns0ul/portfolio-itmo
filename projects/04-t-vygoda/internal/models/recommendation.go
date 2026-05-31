package models

import "time"

type Recommendation struct {
	UserID      int64     `json:"user_id"`
	PromoID     int64     `json:"promo_id"`
	Score       float64   `json:"score"`
	Reason      string    `json:"reason,omitempty"`
	GeneratedAt time.Time `json:"generated_at"`
}

type RecommendationBatch struct {
	UserID  int64            `json:"user_id"`
	Items   []Recommendation `json:"items"`
	Source  string           `json:"source"`
}
