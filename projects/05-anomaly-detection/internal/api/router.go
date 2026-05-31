package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/andrey/anomaly-detection/internal/clickhouse"
)

type Deps struct {
	Alerts *clickhouse.AlertRepo
	CH     *clickhouse.Client
}

func NewRouter(d *Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(requestLogger(), gin.Recovery(), cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "OPTIONS"},
		AllowHeaders: []string{"Origin", "Content-Type", "Accept"},
		MaxAge:       12 * time.Hour,
	}))

	h := &handlers{d: d}
	v1 := r.Group("/api/v1")
	v1.GET("/alerts", h.alerts)
	v1.GET("/alerts/client/:id", h.alertsByClient)
	v1.GET("/stats", h.stats)
	v1.GET("/health", h.health)
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
		)
	}
}

func writeInternal(c *gin.Context, err error) {
	slog.Error("handler", "path", c.FullPath(), "err", err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
}
