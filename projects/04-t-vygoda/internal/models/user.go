package models

import "time"

type User struct {
	ID           int64     `json:"id"`
	Phone        string    `json:"phone"`
	Name         string    `json:"name"`
	Email        *string   `json:"email,omitempty"`
	AvatarURL    *string   `json:"avatar_url,omitempty"`
	ReferralCode string    `json:"referral_code"`
	ReferredBy   *int64    `json:"referred_by,omitempty"`
	Level        int16     `json:"level"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type UserPublic struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	AvatarURL *string `json:"avatar_url,omitempty"`
	Level     int16   `json:"level"`
}

func (u User) Public() UserPublic {
	return UserPublic{ID: u.ID, Name: u.Name, AvatarURL: u.AvatarURL, Level: u.Level}
}

type UserStats struct {
	UserID            int64   `json:"user_id"`
	PostsCount        int64   `json:"posts_count"`
	PurchasesCount    int64   `json:"purchases_count"`
	TotalSpent        float64 `json:"total_spent"`
	ReferralsLevel1   int64   `json:"referrals_level1"`
	ReferralsTotal    int64   `json:"referrals_total"`
	BonusesEarned     float64 `json:"bonuses_earned"`
}
