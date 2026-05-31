package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	agg "github.com/andrey/wallet-scoring/internal/grpcint"
)

type adminHandlers struct {
	agg      *agg.Client
	redis    *redis.Client
	watchKey string
}

func (h *adminHandlers) refresh(c *gin.Context) {
	addr := c.Param("address")
	if !isLikelyPubkey(addr) {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "bad address", Code: ErrCodeBadRequest})
		return
	}
	if h.agg == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: "aggregator not configured", Code: ErrCodeInternal})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if err := h.agg.Refresh(ctx, addr); err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusAccepted, SuccessResponse{Data: gin.H{"wallet": addr, "queued": true}})
}

type watchRequest struct {
	Address string `json:"address"`
	Remove  bool   `json:"remove,omitempty"`
}

func (h *adminHandlers) watch(c *gin.Context) {
	var req watchRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "bad json", Code: ErrCodeBadRequest})
		return
	}
	if !isLikelyPubkey(req.Address) {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "bad address", Code: ErrCodeBadRequest})
		return
	}
	var err error
	if req.Remove {
		err = h.redis.SRem(c.Request.Context(), h.watchKey, req.Address).Err()
	} else {
		err = h.redis.SAdd(c.Request.Context(), h.watchKey, req.Address).Err()
	}
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, SuccessResponse{Data: gin.H{"address": req.Address, "removed": req.Remove}})
}
