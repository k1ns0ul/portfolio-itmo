package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/andrey/wallet-scoring/internal/clickhouse"
	agg "github.com/andrey/wallet-scoring/internal/grpcint"
)

type walletHandlers struct {
	wallets *clickhouse.WalletRepo
	txs     *clickhouse.TxRepo
	agg     *agg.Client
}

func (h *walletHandlers) get(c *gin.Context) {
	addr := c.Param("address")
	if !isLikelyPubkey(addr) {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "bad address", Code: ErrCodeBadRequest})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	if h.agg != nil {
		resp, err := h.agg.WalletStats(ctx, addr)
		if err == nil && resp.Found {
			c.JSON(http.StatusOK, SuccessResponse{Data: resp})
			return
		}
		if err != nil {
			slog.Debug("aggregator fallback", "err", err)
		}
	}

	stats, err := h.wallets.GetStats(c.Request.Context(), addr)
	if errors.Is(err, clickhouse.ErrNotFound) {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "wallet not found", Code: ErrCodeNotFound})
		return
	}
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, SuccessResponse{Data: stats})
}

func (h *walletHandlers) transactions(c *gin.Context) {
	addr := c.Param("address")
	if !isLikelyPubkey(addr) {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "bad address", Code: ErrCodeBadRequest})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	cursor := c.Query("cursor")

	txs, next, err := h.txs.GetByWallet(c.Request.Context(), addr, limit, cursor)
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, PaginatedResponse{Data: txs, Cursor: next, HasMore: next != ""})
}

func (h *walletHandlers) history(c *gin.Context) {
	addr := c.Param("address")
	if !isLikelyPubkey(addr) {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "bad address", Code: ErrCodeBadRequest})
		return
	}
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	if days <= 0 || days > 365 {
		days = 30
	}
	from := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "500"))

	points, err := h.wallets.GetHistory(c.Request.Context(), addr, from, limit)
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, SuccessResponse{Data: points})
}

func isLikelyPubkey(s string) bool {
	if len(s) < 32 || len(s) > 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= '0' && c <= '9') && !(c >= 'A' && c <= 'Z') && !(c >= 'a' && c <= 'z') {
			return false
		}
		if c == '0' || c == 'O' || c == 'I' || c == 'l' {
			return false
		}
	}
	return true
}

func writeInternal(c *gin.Context, err error) {
	slog.Error("handler", "path", c.FullPath(), "err", err)
	c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal error", Code: ErrCodeInternal})
}
