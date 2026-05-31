package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/andrey/cfa-bonds/internal/repo"
)

type pageMeta struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Total  int `json:"total,omitempty"`
}

func sendOK(c *gin.Context, payload any) {
	c.JSON(http.StatusOK, payload)
}

func created(c *gin.Context, payload any) {
	c.JSON(http.StatusCreated, payload)
}

func listResponse(c *gin.Context, items any, meta pageMeta) {
	c.JSON(http.StatusOK, gin.H{"items": items, "page": meta})
}

func badRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, gin.H{"error": msg})
}

func failWith(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repo.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "resource not found"})
	case errors.Is(err, repo.ErrConflict):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func parsePaging(c *gin.Context) (int, int) {
	limit := queryInt(c, "limit", 50)
	offset := queryInt(c, "offset", 0)
	if limit < 1 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}
