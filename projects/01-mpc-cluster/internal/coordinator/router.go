package coordinator

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

func NewRouter(h *Handlers, log *slog.Logger) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(requestLogger(log))

	v1 := r.Group("/api/v1")
	{
		v1.GET("/health", h.Health)
		v1.POST("/sessions", h.CreateSession)
		v1.GET("/sessions", h.ListSessions)
		v1.GET("/sessions/:id", h.GetSession)
		v1.POST("/sessions/:id/execute", h.ExecuteSession)
		v1.DELETE("/sessions/:id", h.DeleteSession)
		v1.GET("/sessions/:id/rounds", h.GetRounds)
	}
	return r
}

func requestLogger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Info("http",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"dur", time.Since(start).String(),
		)
	}
}
