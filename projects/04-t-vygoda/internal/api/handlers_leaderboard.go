package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	rds "github.com/andrey/t-vygoda/internal/redis"
)

type leaderboardHandlers struct {
	d *Deps
}

func (h *leaderboardHandlers) top(c *gin.Context) {
	name := rds.LeaderboardName(c.Param("type"))
	if !name.Valid() {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "bad leaderboard type", Code: ErrCodeBadRequest})
		return
	}
	n, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	entries, err := h.d.Leaderboard.TopN(c.Request.Context(), name, n)
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, SuccessResponse{Data: entries})
}
