package api

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/andrey/wallet-scoring/internal/ratelimit"
)

func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		slog.Info("http",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"dur_ms", time.Since(start).Milliseconds(),
			"client", c.ClientIP(),
			"size", c.Writer.Size(),
		)
	}
}

func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		slog.Error("panic", "value", recovered, "path", c.Request.URL.Path)
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorResponse{
			Error: "internal error",
			Code:  ErrCodeInternal,
		})
	})
}

func RateLimit(limiter *ratelimit.Limiter, perMinute int) gin.HandlerFunc {
	if perMinute <= 0 {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		ok, remaining, err := limiter.Allow(c.Request.Context(), c.ClientIP(), perMinute, time.Minute)
		if err != nil {
			slog.Error("rate limit", "err", err)
			c.Next()
			return
		}
		c.Header("X-RateLimit-Limit", strconv.Itoa(perMinute))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		if !ok {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, ErrorResponse{
				Error: "too many requests",
				Code:  ErrCodeRateLimited,
			})
			return
		}
		c.Next()
	}
}

func APIKeyAuth(rdb *redis.Client, required bool, setName string) gin.HandlerFunc {
	if !required {
		return func(c *gin.Context) { c.Next() }
	}
	if setName == "" {
		setName = "api-keys"
	}
	return func(c *gin.Context) {
		key := c.GetHeader("X-API-Key")
		if key == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{
				Error: "missing api key",
				Code:  ErrCodeUnauthorized,
			})
			return
		}
		isMember, err := rdb.SIsMember(c.Request.Context(), setName, key).Result()
		if err != nil {
			slog.Error("api key check", "err", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorResponse{
				Error: "auth backend error",
				Code:  ErrCodeInternal,
			})
			return
		}
		if !isMember {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{
				Error: "invalid api key",
				Code:  ErrCodeUnauthorized,
			})
			return
		}
		c.Next()
	}
}
