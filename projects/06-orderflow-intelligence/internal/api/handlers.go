package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	rds "github.com/andrey/orderflow-intelligence/internal/redis"
)

type handlers struct {
	d *Deps
}

func (h *handlers) pairs(c *gin.Context) {
	intervalSec := parseIntervalSec(c.DefaultQuery("interval", "1m"))
	list, err := h.d.Repo.GetPairStats(c.Request.Context(), intervalSec)
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": list, "interval_sec": intervalSec})
}

func (h *handlers) features(c *gin.Context) {
	pair := c.Param("pair")
	if pair == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing pair"})
		return
	}
	intervalSec := parseIntervalSec(c.DefaultQuery("interval", "1m"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	windows, err := h.d.Repo.GetLatestByPair(c.Request.Context(), pair, intervalSec, limit)
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": windows, "count": len(windows)})
}

func (h *handlers) predictions(c *gin.Context) {
	pair := c.Param("pair")
	if pair == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing pair"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	preds, err := h.d.Repo.GetPredictions(c.Request.Context(), pair, limit)
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": preds, "count": len(preds)})
}

func (h *handlers) latest(c *gin.Context) {
	pair := c.Param("pair")
	if pair == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing pair"})
		return
	}
	intervalSec := parseIntervalSec(c.DefaultQuery("interval", "1m"))

	window, err := h.d.Cache.GetLatestWindow(c.Request.Context(), pair, intervalSec)
	if err != nil && !errors.Is(err, rds.ErrCacheMiss) {
		writeInternal(c, err)
		return
	}
	prediction, err := h.d.Cache.GetPrediction(c.Request.Context(), pair)
	if err != nil && !errors.Is(err, rds.ErrCacheMiss) {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"pair":         pair,
		"interval_sec": intervalSec,
		"window":       window,
		"prediction":   prediction,
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

func parseIntervalSec(raw string) int {
	if d, err := time.ParseDuration(raw); err == nil && d > 0 {
		return int(d.Seconds())
	}
	if n, err := strconv.Atoi(raw); err == nil && n > 0 {
		return n
	}
	return 60
}
