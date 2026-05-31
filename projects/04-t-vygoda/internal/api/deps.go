package api

import (
	"github.com/redis/go-redis/v9"

	"github.com/andrey/t-vygoda/internal/auth"
	"github.com/andrey/t-vygoda/internal/config"
	"github.com/andrey/t-vygoda/internal/kafka"
	rds "github.com/andrey/t-vygoda/internal/redis"
	"github.com/andrey/t-vygoda/internal/recommender"
	"github.com/andrey/t-vygoda/internal/repo"
)

type Deps struct {
	Cfg            config.Config
	Tokenizer      *auth.Tokenizer
	Users          *repo.UserRepo
	Posts          *repo.PostRepo
	Promos         *repo.PromoRepo
	Partners       *repo.PartnerRepo
	Purchases      *repo.PurchaseRepo
	Referrals      *repo.ReferralRepo
	CFA            *repo.CFARepo
	Categories     *repo.CategoryRepo
	Recommendations *repo.RecommendationRepo
	Producer       *kafka.Producer
	Redis          *redis.Client
	Cache          *rds.Cache
	Streaks        *rds.Streaks
	Leaderboard    *rds.Leaderboard
	RecsCache      *rds.Recommendations
	Recommender    *recommender.Client
	VisitCh        chan<- int64
}
