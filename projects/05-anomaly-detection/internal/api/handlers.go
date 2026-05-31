package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type handlers struct {
	d *Deps
}

func (h *handlers) alerts(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	alerts, err := h.d.Alerts.GetRecent(c.Request.Context(), limit)
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": alerts, "count": len(alerts)})
}

func (h *handlers) alertsByClient(c *gin.Context) {
	clientID := c.Param("id")
	if clientID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing client id"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	alerts, err := h.d.Alerts.GetByClient(c.Request.Context(), clientID, limit)
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": alerts, "count": len(alerts)})
}

func (h *handlers) stats(c *gin.Context) {
	ctx := c.Request.Context()
	hour, err := h.d.Alerts.GetStats(ctx, time.Hour)
	if err != nil {
		writeInternal(c, err)
		return
	}
	day, err := h.d.Alerts.GetStats(ctx, 24*time.Hour)
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"last_hour": hour,
		"last_24h":  day,
		"generated_at": time.Now().UTC(),
	})
}

func (h *handlers) health(c *gin.Context) {
	if err := h.d.CH.Ping(c.Request.Context()); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"clickhouse": "down", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
