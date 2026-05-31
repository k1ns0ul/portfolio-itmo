package models

import "time"

type UserStreak struct {
	UserID        int64     `json:"user_id"`
	CurrentStreak int       `json:"current_streak"`
	LongestStreak int       `json:"longest_streak"`
	LastVisit     time.Time `json:"last_visit"`
}
