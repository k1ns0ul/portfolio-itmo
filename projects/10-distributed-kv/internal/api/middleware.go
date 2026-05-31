package api

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

type Metrics interface {
	Observe(method string, status int, seconds float64)
}

func requestID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(buf)
}

func RequestLogger(log *slog.Logger, m Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		rid := requestID()
		c.Set("request_id", rid)
		c.Next()

		dur := time.Since(start)
		if m != nil {
			m.Observe(c.Request.Method, c.Writer.Status(), dur.Seconds())
		}
		log.Info("request",
			"request_id", rid,
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration", dur.String(),
		)
	}
}
