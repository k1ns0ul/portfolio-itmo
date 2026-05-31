package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/andrey/t-vygoda/internal/auth"
)

func NewRouter(d *Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(requestLogger(), recovery(), cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	ah := &authHandlers{d: d}
	ph := &postHandlers{d: d}
	prh := &promoHandlers{d: d}
	pch := &purchaseHandlers{d: d}
	uh := &userHandlers{d: d}
	pnh := &partnerHandlers{d: d}
	fh := &feedHandlers{d: d}
	lh := &leaderboardHandlers{d: d}
	adh := &adminHandlers{d: d}

	v1 := r.Group("/api/v1")

	v1.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, SuccessResponse{Data: gin.H{"status": "ok"}})
	})

	pub := v1.Group("/auth")
	pub.POST("/register", ah.register)
	pub.POST("/login", ah.login)
	pub.POST("/refresh", ah.refresh)

	authed := v1.Group("")
	authed.Use(auth.Middleware(d.Tokenizer), visitTracker(d))

	authed.GET("/users/me", uh.me)
	authed.PUT("/users/me", uh.updateMe)
	authed.GET("/users/me/referrals", uh.referrals)
	authed.GET("/users/me/bonuses", uh.bonuses)
	authed.GET("/users/me/streak", uh.streak)
	authed.GET("/users/:id", uh.get)
	authed.GET("/users/:id/posts", ph.listByUser)

	authed.POST("/posts", ph.create)
	authed.GET("/posts/:id", ph.get)
	authed.GET("/posts", ph.list)
	authed.POST("/posts/:id/like", ph.like)
	authed.DELETE("/posts/:id/like", ph.unlike)

	authed.GET("/promos", prh.list)
	authed.GET("/promos/popular", prh.popular)
	authed.GET("/promos/:id", prh.get)
	authed.POST("/promos/:id/activate", prh.activate)

	authed.POST("/purchases", pch.create)
	authed.GET("/purchases", pch.list)
	authed.POST("/purchases/:id/confirm", pch.confirm)

	authed.GET("/partners", pnh.list)
	authed.GET("/partners/:id", pnh.get)
	authed.GET("/partners/:id/balance", pnh.balance)

	authed.GET("/feed", fh.personalized)
	authed.GET("/leaderboard/:type", lh.top)

	admin := authed.Group("/admin")
	admin.POST("/partners", pnh.create)
	admin.PUT("/partners/:id", pnh.update)
	admin.POST("/cfa/settle/:partner_id", adh.cfaSettle)
	admin.GET("/cfa/balances", adh.cfaBalances)
	admin.GET("/stats", adh.stats)
	admin.POST("/recommendations/refresh", adh.refreshRecs)

	return r
}

func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		slog.Info("http",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"dur_ms", time.Since(start).Milliseconds(),
			"client", c.ClientIP(),
		)
	}
}

func recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		slog.Error("panic", "value", recovered, "path", c.Request.URL.Path)
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorResponse{
			Error: "internal error", Code: ErrCodeInternal,
		})
	})
}

func visitTracker(d *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := auth.UserID(c)
		if uid > 0 && d.VisitCh != nil {
			select {
			case d.VisitCh <- uid:
			default:
			}
		}
		c.Next()
	}
}
