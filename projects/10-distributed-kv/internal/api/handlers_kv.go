package api

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/andrey/distributed-kv/internal/store"
)

func (s *Server) putKey(c *gin.Context) {
	key := c.Param("key")
	value, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "read body: " + err.Error()})
		return
	}

	if !s.node.IsLeader() {
		s.forwardWrite(c, value)
		return
	}

	cmd := store.Command{Op: store.OpSet, Key: key, Value: value}
	if err := s.node.Apply(cmd, applyTimeout); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"key": key, "status": "stored"})
}

func (s *Server) deleteKey(c *gin.Context) {
	key := c.Param("key")

	if !s.node.IsLeader() {
		s.forwardWrite(c, nil)
		return
	}

	cmd := store.Command{Op: store.OpDelete, Key: key}
	if err := s.node.Apply(cmd, applyTimeout); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"key": key, "status": "deleted"})
}

func (s *Server) getKey(c *gin.Context) {
	key := c.Param("key")
	consistent := c.Query("consistent") == "true"

	if consistent {
		if err := s.node.VerifyLeader(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "consistent read requires leader: " + err.Error()})
			return
		}
	}

	value, ok := s.store.Get(key)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "key not found", "key": key})
		return
	}
	c.Data(http.StatusOK, "application/octet-stream", value)
}

func (s *Server) listKeys(c *gin.Context) {
	prefix := c.Query("prefix")
	keys := s.store.Keys(prefix)
	c.JSON(http.StatusOK, gin.H{"keys": keys, "count": len(keys)})
}

func (s *Server) forwardWrite(c *gin.Context, body []byte) {
	if isForwarded(c) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "forwarded request reached a non-leader node"})
		return
	}
	status, respBody, err := s.forwarder.ForwardToLeader(c.Request.Context(), c.Request.Method, c.Request.URL.RequestURI(), body)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	c.Data(status, "application/json", respBody)
}
