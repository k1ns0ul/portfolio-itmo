package api

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/andrey/wallet-scoring/internal/clickhouse"
	"github.com/andrey/wallet-scoring/internal/config"
	agg "github.com/andrey/wallet-scoring/internal/grpcint"
	"github.com/andrey/wallet-scoring/internal/ratelimit"
)

type Deps struct {
	Config     config.APIConfig
	TxRepo     *clickhouse.TxRepo
	WalletRepo *clickhouse.WalletRepo
	TokenRepo  *clickhouse.TokenRepo
	Aggregator *agg.Client
	Redis      *redis.Client
}

func NewRouter(d Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	limiter := ratelimit.New(d.Redis, "api")

	r.Use(
		RequestLogger(),
		Recovery(),
		cors.New(cors.Config{
			AllowOrigins:     []string{"*"},
			AllowMethods:     []string{"GET", "POST", "OPTIONS"},
			AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "X-API-Key"},
			ExposeHeaders:    []string{"X-RateLimit-Limit", "X-RateLimit-Remaining"},
			AllowCredentials: false,
			MaxAge:           12 * time.Hour,
		}),
		RateLimit(limiter, d.Config.RateLimitPerMinute),
		APIKeyAuth(d.Redis, d.Config.APIKeyRequired, "api-keys"),
	)

	hub := NewHub(d.Redis, d.Config.WebsocketReadTimeout, d.Config.WebsocketWriteTimeout)
	wh := &walletHandlers{wallets: d.WalletRepo, txs: d.TxRepo, agg: d.Aggregator}
	th := &tokenHandlers{tokens: d.TokenRepo}
	sh := &statsHandlers{wallets: d.WalletRepo, txs: d.TxRepo}
	srh := &searchHandlers{wallets: d.WalletRepo}
	ah := &adminHandlers{agg: d.Aggregator, redis: d.Redis, watchKey: "watchlist"}

	v1 := r.Group("/api/v1")
	v1.GET("/wallet/:address", wh.get)
	v1.GET("/wallet/:address/transactions", wh.transactions)
	v1.GET("/wallet/:address/history", wh.history)
	v1.GET("/token/:mint", th.get)
	v1.GET("/tokens/suspicious", th.suspicious)
	v1.GET("/stats", sh.global)
	v1.GET("/stats/distribution", sh.distribution)
	v1.GET("/search", srh.search)
	v1.GET("/health", sh.health)

	admin := v1.Group("/admin")
	admin.POST("/refresh/:address", ah.refresh)
	admin.POST("/watch", ah.watch)

	v1.GET("/ws", hub.handle)
	go hub.run()

	return r
}
