package api

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/andrey/t-vygoda/internal/models"
	"github.com/andrey/t-vygoda/internal/repo"
)

type adminHandlers struct {
	d *Deps
}

func (h *adminHandlers) cfaSettle(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("partner_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "bad partner id", Code: ErrCodeBadRequest})
		return
	}
	rec, err := h.d.CFA.Reconcile(c.Request.Context(), id)
	if errors.Is(err, repo.ErrNotFound) {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "balance not found", Code: ErrCodeNotFound})
		return
	}
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, SuccessResponse{Data: rec})
}

func (h *adminHandlers) cfaBalances(c *gin.Context) {
	list, err := h.d.CFA.ListBalances(c.Request.Context())
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, SuccessResponse{Data: list})
}

func (h *adminHandlers) stats(c *gin.Context) {
	ctx := c.Request.Context()
	now := time.Now().UTC()
	from := now.Add(-30 * 24 * time.Hour)

	type stats struct {
		Users        uint64              `json:"users"`
		Purchases30d uint64              `json:"purchases_30d"`
		ActivePromos int                 `json:"active_promos"`
		AvgCheck     float64             `json:"avg_check_30d"`
		Revenue30d   float64             `json:"revenue_30d"`
		PerPartner   []models.CFABalance `json:"partner_balances"`
		GeneratedAt  time.Time           `json:"generated_at"`
	}
	s := stats{GeneratedAt: now}

	if n, err := h.d.Users.Count(ctx); err != nil {
		slog.Error("admin stats: users count", "err", err)
	} else {
		s.Users = n
	}
	if rows, err := h.d.Promos.ListActive(ctx, nil, 1000); err != nil {
		slog.Error("admin stats: active promos", "err", err)
	} else {
		s.ActivePromos = len(rows)
	}
	if balances, err := h.d.CFA.ListBalances(ctx); err != nil {
		slog.Error("admin stats: balances", "err", err)
	} else {
		s.PerPartner = balances
	}
	if sum, err := h.d.Purchases.ConfirmedSummary(ctx, from, now); err != nil {
		slog.Error("admin stats: purchases summary", "err", err)
	} else {
		s.Purchases30d = sum.Count
		s.Revenue30d = sum.Total
		s.AvgCheck = sum.AvgCheck
	}

	c.JSON(http.StatusOK, SuccessResponse{Data: s})
}

type refreshRecsInput struct {
	UserID  int64 `json:"user_id"`
	AllUsers bool  `json:"all_users"`
}

func (h *adminHandlers) refreshRecs(c *gin.Context) {
	var in refreshRecsInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: ErrCodeBadRequest})
		return
	}
	if in.UserID == 0 && !in.AllUsers {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "specify user_id or all_users", Code: ErrCodeBadRequest})
		return
	}
	if in.AllUsers {
		c.JSON(http.StatusAccepted, SuccessResponse{Data: gin.H{"queued": true, "scope": "all"}})
		return
	}
	if err := h.d.RecsCache.Invalidate(c.Request.Context(), in.UserID); err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusAccepted, SuccessResponse{Data: gin.H{"queued": true, "user_id": in.UserID}})
}
