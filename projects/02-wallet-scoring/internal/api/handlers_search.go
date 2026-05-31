package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/andrey/wallet-scoring/internal/clickhouse"
)

type searchHandlers struct {
	wallets *clickhouse.WalletRepo
}

func (h *searchHandlers) search(c *gin.Context) {
	q := c.Query("q")
	if len(q) < 3 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "query must be at least 3 chars", Code: ErrCodeBadRequest})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	list, err := h.wallets.Search(c.Request.Context(), q, limit)
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, SuccessResponse{Data: list})
}
