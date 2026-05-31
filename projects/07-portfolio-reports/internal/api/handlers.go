package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	rds "github.com/andrey/portfolio-reports/internal/redis"
	"github.com/andrey/portfolio-reports/internal/models"
)

type handlers struct {
	d *Deps
}

type reportRequest struct {
	Address string `json:"address" binding:"required,min=32,max=64"`
}

type batchRequest struct {
	Addresses []string `json:"addresses" binding:"required,min=1,max=50"`
}

func (h *handlers) generate(c *gin.Context) {
	var in reportRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	report, source, err := h.generateOne(c.Request.Context(), in.Address)
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": report, "source": source})
}

func (h *handlers) getCached(c *gin.Context) {
	address := c.Param("address")
	if address == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing address"})
		return
	}
	cached, err := h.d.Cache.GetReport(c.Request.Context(), address)
	if err == nil {
		c.JSON(http.StatusOK, gin.H{"data": cached, "source": "cache"})
		return
	}
	if !errors.Is(err, rds.ErrCacheMiss) {
		slog.Warn("cache read", "err", err)
	}
	report, source, err := h.generateOne(c.Request.Context(), address)
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": report, "source": source})
}

type batchItem struct {
	Address string         `json:"address"`
	Report  *models.Report `json:"report,omitempty"`
	Source  string         `json:"source,omitempty"`
	Error   string         `json:"error,omitempty"`
}

func (h *handlers) batch(c *gin.Context) {
	var in batchRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	results := make([]batchItem, len(in.Addresses))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for i, addr := range in.Addresses {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, addr string) {
			defer wg.Done()
			defer func() { <-sem }()
			ctx, cancel := context.WithTimeout(c.Request.Context(), h.d.Cfg.Timeout)
			defer cancel()
			report, source, err := h.generateOne(ctx, addr)
			results[i] = batchItem{Address: addr}
			if err != nil {
				results[i].Error = err.Error()
				return
			}
			results[i].Report = report
			results[i].Source = source
		}(i, addr)
	}
	wg.Wait()
	c.JSON(http.StatusOK, gin.H{"data": results})
}

func (h *handlers) health(c *gin.Context) {
	if err := h.d.CH.Ping(c.Request.Context()); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"clickhouse": "down", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *handlers) generateOne(ctx context.Context, address string) (*models.Report, string, error) {
	if cached, err := h.d.Cache.GetReport(ctx, address); err == nil {
		return cached, "cache", nil
	} else if !errors.Is(err, rds.ErrCacheMiss) {
		slog.Warn("cache read", "err", err)
	}

	portfolio, err := h.d.Metrics.CalculatePortfolio(ctx, address)
	if err != nil {
		return nil, "", err
	}
	resp, err := h.d.LLM.Generate(ctx, portfolio)
	if err != nil {
		return nil, "", err
	}
	report := &models.Report{
		Address:     address,
		GeneratedAt: time.Now().UTC(),
		Metrics:     *portfolio,
		TextReport:  resp.Text,
		Summary:     resp.Summary,
		Source:      resp.Source,
	}
	if err := h.d.Cache.SetReport(ctx, address, report, h.d.Cfg.CacheTTL); err != nil {
		slog.Warn("cache write", "err", err)
	}
	return report, resp.Source, nil
}
