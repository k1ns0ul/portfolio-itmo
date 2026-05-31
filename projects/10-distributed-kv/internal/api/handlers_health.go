package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) health(c *gin.Context) {
	state := s.node.State()
	if state == "Shutdown" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "down", "state": state})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "state": state})
}

func (s *Server) ready(c *gin.Context) {
	if s.node.AppliedIndex() == 0 {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "reason": "no entries applied yet"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready", "applied_index": s.node.AppliedIndex()})
}
