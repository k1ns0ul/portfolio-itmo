package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/andrey/t-vygoda/internal/models"
	"github.com/andrey/t-vygoda/internal/repo"
)

type partnerHandlers struct {
	d *Deps
}

func (h *partnerHandlers) list(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	list, err := h.d.Partners.ListActive(c.Request.Context(), limit)
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, SuccessResponse{Data: list})
}

func (h *partnerHandlers) get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "bad id", Code: ErrCodeBadRequest})
		return
	}
	p, err := h.d.Partners.GetByID(c.Request.Context(), id)
	if errors.Is(err, repo.ErrNotFound) {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "partner not found", Code: ErrCodeNotFound})
		return
	}
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, SuccessResponse{Data: p})
}

func (h *partnerHandlers) balance(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "bad id", Code: ErrCodeBadRequest})
		return
	}
	out, err := h.d.Partners.WithBalance(c.Request.Context(), id)
	if errors.Is(err, repo.ErrNotFound) {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "partner not found", Code: ErrCodeNotFound})
		return
	}
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, SuccessResponse{Data: out})
}

func (h *partnerHandlers) create(c *gin.Context) {
	var in models.PartnerInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: ErrCodeBadRequest})
		return
	}
	p, err := h.d.Partners.Create(c.Request.Context(), in)
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusCreated, SuccessResponse{Data: p})
}

func (h *partnerHandlers) update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "bad id", Code: ErrCodeBadRequest})
		return
	}
	var in models.PartnerInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: ErrCodeBadRequest})
		return
	}
	p, err := h.d.Partners.Update(c.Request.Context(), id, in)
	if errors.Is(err, repo.ErrNotFound) {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "partner not found", Code: ErrCodeNotFound})
		return
	}
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, SuccessResponse{Data: p})
}
