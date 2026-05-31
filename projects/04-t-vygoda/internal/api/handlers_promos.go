package api

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/andrey/t-vygoda/internal/auth"
	"github.com/andrey/t-vygoda/internal/models"
	"github.com/andrey/t-vygoda/internal/repo"
)

type promoHandlers struct {
	d *Deps
}

func (h *promoHandlers) list(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	var categoryID *int64
	if raw := c.Query("category"); raw != "" {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil {
			categoryID = &id
		}
	}
	list, err := h.d.Promos.ListActive(c.Request.Context(), categoryID, limit)
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, SuccessResponse{Data: list})
}

func (h *promoHandlers) get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "bad id", Code: ErrCodeBadRequest})
		return
	}
	p, err := h.d.Promos.GetByID(c.Request.Context(), id)
	if errors.Is(err, repo.ErrNotFound) {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "promo not found", Code: ErrCodeNotFound})
		return
	}
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, SuccessResponse{Data: p})
}

func (h *promoHandlers) popular(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	list, err := h.d.Promos.Popular(c.Request.Context(), limit)
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, SuccessResponse{Data: list})
}

func (h *promoHandlers) activate(c *gin.Context) {
	uid, ok := auth.MustUserID(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "bad id", Code: ErrCodeBadRequest})
		return
	}
	p, err := h.d.Promos.GetByID(c.Request.Context(), id)
	if errors.Is(err, repo.ErrNotFound) {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "promo not found", Code: ErrCodeNotFound})
		return
	}
	if err != nil {
		writeInternal(c, err)
		return
	}
	if !p.Available() {
		c.JSON(http.StatusConflict, ErrorResponse{Error: "promo unavailable", Code: ErrCodeUnavailable})
		return
	}

	ev, err := models.NewEvent(models.EventPromoActivated, "api", gin.H{
		"promo_id": p.ID, "code": p.Code, "user_id": uid, "partner_id": p.PartnerID,
	})
	if err != nil {
		writeInternal(c, err)
		return
	}
	if err := h.d.Producer.Publish(h.d.Cfg.Kafka.TopicPromos, strconv.FormatInt(p.ID, 10), ev); err != nil {
		slog.Error("publish promo activated", "err", err, "promo_id", p.ID)
	}
	c.JSON(http.StatusAccepted, SuccessResponse{Data: gin.H{"promo_id": p.ID, "queued": true}})
}
