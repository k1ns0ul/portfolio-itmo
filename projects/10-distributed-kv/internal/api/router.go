package api

import (
	"github.com/gin-gonic/gin"
)

func (s *Server) buildRouter(m Metrics) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(RequestLogger(s.log, m))

	v1 := r.Group("/api/v1")
	{
		v1.GET("/health", s.health)
		v1.GET("/ready", s.ready)

		v1.PUT("/kv/:key", s.putKey)
		v1.GET("/kv/:key", s.getKey)
		v1.DELETE("/kv/:key", s.deleteKey)
		v1.GET("/kv", s.listKeys)

		v1.GET("/cluster/status", s.clusterStatus)
		v1.POST("/cluster/join", s.clusterJoin)
		v1.POST("/cluster/remove", s.clusterRemove)
		v1.GET("/cluster/peers", s.clusterPeers)
	}
	return r
}

func isForwarded(c *gin.Context) bool {
	return c.GetHeader("X-Forwarded-Internal") == "1"
}
