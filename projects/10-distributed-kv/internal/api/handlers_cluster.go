package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func (s *Server) clusterStatus(c *gin.Context) {
	stats := s.node.Stats()
	servers, _ := s.node.Servers()
	peers := make([]string, 0, len(servers))
	for _, srv := range servers {
		peers = append(peers, string(srv.ID)+"@"+string(srv.Address))
	}

	c.JSON(http.StatusOK, gin.H{
		"id":            s.node.ID(),
		"state":         s.node.State(),
		"leader":        s.node.Leader(),
		"term":          stats["term"],
		"commit_index":  stats["commit_index"],
		"applied_index": strconv.FormatUint(s.node.AppliedIndex(), 10),
		"peers":         peers,
		"fsm_key_count": s.store.Count(),
	})
}

type joinReq struct {
	ID   string `json:"id" binding:"required"`
	Addr string `json:"addr" binding:"required"`
}

func (s *Server) clusterJoin(c *gin.Context) {
	var req joinReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid join payload: " + err.Error()})
		return
	}
	if !s.node.IsLeader() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "join must be sent to the leader", "leader": s.node.Leader()})
		return
	}
	if err := s.node.AddVoter(req.ID, req.Addr); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "joined", "id": req.ID})
}

type removeReq struct {
	ID string `json:"id" binding:"required"`
}

func (s *Server) clusterRemove(c *gin.Context) {
	var req removeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid remove payload: " + err.Error()})
		return
	}
	if !s.node.IsLeader() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "remove must be sent to the leader", "leader": s.node.Leader()})
		return
	}
	if err := s.node.RemoveServer(req.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "removed", "id": req.ID})
}

func (s *Server) clusterPeers(c *gin.Context) {
	servers, err := s.node.Servers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	leader := s.node.Leader()
	peers := make([]gin.H, 0, len(servers))
	for _, srv := range servers {
		peers = append(peers, gin.H{
			"id":        string(srv.ID),
			"address":   string(srv.Address),
			"is_leader": string(srv.Address) == leader,
		})
	}
	c.JSON(http.StatusOK, gin.H{"peers": peers})
}
