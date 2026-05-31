package models

import "time"

type Post struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	ImageURL    *string   `json:"image_url,omitempty"`
	PriceBefore *float64  `json:"price_before,omitempty"`
	PriceAfter  *float64  `json:"price_after,omitempty"`
	PromoID     *int64    `json:"promo_id,omitempty"`
	CategoryID  *int64    `json:"category_id,omitempty"`
	LikesCount  int       `json:"likes_count"`
	CreatedAt   time.Time `json:"created_at"`
}

type PostWithUser struct {
	Post
	Author UserPublic `json:"author"`
	Liked  bool       `json:"liked"`
}

type FeedSort string

const (
	FeedSortRecent      FeedSort = "recent"
	FeedSortPopular     FeedSort = "popular"
	FeedSortRecommended FeedSort = "recommended"
)

func (s FeedSort) Valid() bool {
	switch s {
	case FeedSortRecent, FeedSortPopular, FeedSortRecommended:
		return true
	}
	return false
}

type CreatePostInput struct {
	Title       string   `json:"title" binding:"required,min=3,max=200"`
	Description string   `json:"description" binding:"max=4000"`
	ImageURL    *string  `json:"image_url,omitempty"`
	PriceBefore *float64 `json:"price_before,omitempty"`
	PriceAfter  *float64 `json:"price_after,omitempty"`
	PromoID     *int64   `json:"promo_id,omitempty"`
	CategoryID  *int64   `json:"category_id,omitempty"`
}
