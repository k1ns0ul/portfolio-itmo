package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/andrey/wallet-scoring/internal/clickhouse"
)

type statsHandlers struct {
	wallets *clickhouse.WalletRepo
	txs     *clickhouse.TxRepo
}

func (h *statsHandlers) global(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	wallets, err := h.wallets.GlobalCount(ctx)
	if err != nil {
		writeInternal(c, err)
		return
	}
	to := time.Now().UTC()
	from := to.Add(-24 * time.Hour)
	txs, err := h.txs.CountByTimeRange(ctx, from, to)
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, SuccessResponse{Data: gin.H{
		"wallets":       wallets,
		"txs_last_24h":  txs,
		"generated_at":  time.Now().UTC(),
	}})
}

func (h *statsHandlers) distribution(c *gin.Context) {
	dist, err := h.wallets.GetScoreDistribution(c.Request.Context())
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, SuccessResponse{Data: dist})
}

func (h *statsHandlers) health(c *gin.Context) {
	c.JSON(http.StatusOK, SuccessResponse{Data: gin.H{"status": "ok", "ts": time.Now().UTC()}})
}
