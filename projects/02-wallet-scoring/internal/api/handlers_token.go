package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/andrey/wallet-scoring/internal/clickhouse"
)

type tokenHandlers struct {
	tokens *clickhouse.TokenRepo
}

func (h *tokenHandlers) get(c *gin.Context) {
	mint := c.Param("mint")
	if !isLikelyPubkey(mint) {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "bad mint", Code: ErrCodeBadRequest})
		return
	}
	ts, err := h.tokens.Get(c.Request.Context(), mint)
	if errors.Is(err, clickhouse.ErrNotFound) {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "token not found", Code: ErrCodeNotFound})
		return
	}
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, SuccessResponse{Data: ts})
}

func (h *tokenHandlers) suspicious(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	list, err := h.tokens.Suspicious(c.Request.Context(), limit)
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, SuccessResponse{Data: list})
}
