package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/andrey/cfa-bonds/internal/auth"
	"github.com/andrey/cfa-bonds/internal/redis"
)

func requestLogger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		c.Next()
		log.Info("request",
			"method", c.Request.Method,
			"path", path,
			"status", c.Writer.Status(),
			"size", c.Writer.Size(),
			"latency", time.Since(start).String(),
			"client", c.ClientIP(),
		)
	}
}

func rateLimit(limiter *redis.RateLimiter, log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if limiter == nil {
			c.Next()
			return
		}
		subject := c.ClientIP()
		if claims, ok := auth.FromContext(c); ok {
			subject = claims.InvestorID.String()
		}
		allowed, err := limiter.Allow(c.Request.Context(), subject)
		if err != nil {
			log.Warn("rate limiter unavailable, allowing", "err", err)
			c.Next()
			return
		}
		if !allowed {
			c.Header("Retry-After", "1")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}
