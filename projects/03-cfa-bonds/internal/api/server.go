package api

import (
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/andrey/cfa-bonds/internal/auth"
	"github.com/andrey/cfa-bonds/internal/config"
	"github.com/andrey/cfa-bonds/internal/kafka"
	"github.com/andrey/cfa-bonds/internal/redis"
	"github.com/andrey/cfa-bonds/internal/repo"
)

type Server struct {
	cfg       config.APIConfig
	pool      *pgxpool.Pool
	issuers   *repo.IssuerRepo
	investors *repo.InvestorRepo
	issues    *repo.IssueRepo
	positions *repo.PositionRepo
	trades    *repo.TradeRepo
	coupons   *repo.CouponRepo
	events    *repo.EventRepo
	cache     *redis.Client
	limiter   *redis.RateLimiter
	producer  *kafka.Producer
	auth      *auth.Issuer
	log       *slog.Logger
}

type ServerDeps struct {
	Cfg       config.APIConfig
	Pool      *pgxpool.Pool
	Issuers   *repo.IssuerRepo
	Investors *repo.InvestorRepo
	Issues    *repo.IssueRepo
	Positions *repo.PositionRepo
	Trades    *repo.TradeRepo
	Coupons   *repo.CouponRepo
	Events    *repo.EventRepo
	Cache     *redis.Client
	Limiter   *redis.RateLimiter
	Producer  *kafka.Producer
	Auth      *auth.Issuer
	Log       *slog.Logger
}

func NewServer(d ServerDeps) *Server {
	return &Server{
		cfg:       d.Cfg,
		pool:      d.Pool,
		issuers:   d.Issuers,
		investors: d.Investors,
		issues:    d.Issues,
		positions: d.Positions,
		trades:    d.Trades,
		coupons:   d.Coupons,
		events:    d.Events,
		cache:     d.Cache,
		limiter:   d.Limiter,
		producer:  d.Producer,
		auth:      d.Auth,
		log:       d.Log,
	}
}

func queryInt(c *gin.Context, key string, def int) int {
	v := c.Query(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func pathUUID(c *gin.Context, key string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(key))
	if err != nil {
		badRequest(c, "invalid "+key)
		return uuid.Nil, false
	}
	return id, true
}
