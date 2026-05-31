package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func (s *Server) health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	checks := map[string]string{}
	status := http.StatusOK

	if err := s.pool.Ping(ctx); err != nil {
		checks["postgres"] = "down: " + err.Error()
		status = http.StatusServiceUnavailable
	} else {
		checks["postgres"] = "ok"
	}

	if s.cache != nil {
		if err := s.cache.Ping(ctx); err != nil {
			checks["redis"] = "down: " + err.Error()
			status = http.StatusServiceUnavailable
		} else {
			checks["redis"] = "ok"
		}
	} else {
		checks["redis"] = "disabled"
	}

	if s.producer != nil {
		checks["kafka"] = "ok"
	} else {
		checks["kafka"] = "disabled"
	}

	c.JSON(status, gin.H{"status": statusText(status), "checks": checks})
}

func (s *Server) ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	if err := s.pool.Ping(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "reason": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}

func statusText(code int) string {
	if code == http.StatusOK {
		return "healthy"
	}
	return "degraded"
}
