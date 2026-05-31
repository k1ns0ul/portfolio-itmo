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

type purchaseHandlers struct {
	d *Deps
}

func (h *purchaseHandlers) create(c *gin.Context) {
	uid, ok := auth.MustUserID(c)
	if !ok {
		return
	}
	var in models.CreatePurchaseInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: ErrCodeBadRequest})
		return
	}
	promo, err := h.d.Promos.GetByID(c.Request.Context(), in.PromoID)
	if errors.Is(err, repo.ErrNotFound) {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "promo not found", Code: ErrCodeNotFound})
		return
	}
	if err != nil {
		writeInternal(c, err)
		return
	}
	if !promo.Available() {
		c.JSON(http.StatusConflict, ErrorResponse{Error: "promo unavailable", Code: ErrCodeUnavailable})
		return
	}
	purchase, err := h.d.Purchases.Create(c.Request.Context(), uid, promo.ID, promo.PartnerID, in.Amount)
	if err != nil {
		writeInternal(c, err)
		return
	}

	ev, err := models.NewEvent(models.EventPurchaseCreated, "api", purchase)
	if err != nil {
		slog.Error("build purchase.created event", "err", err)
	} else if err := h.d.Producer.Publish(h.d.Cfg.Kafka.TopicPurchases, strconv.FormatInt(purchase.ID, 10), ev); err != nil {
		slog.Error("publish purchase.created", "err", err, "purchase_id", purchase.ID)
	}

	c.JSON(http.StatusCreated, SuccessResponse{Data: purchase})
}

func (h *purchaseHandlers) confirm(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "bad id", Code: ErrCodeBadRequest})
		return
	}
	purchase, err := h.d.Purchases.GetByID(c.Request.Context(), id)
	if errors.Is(err, repo.ErrNotFound) {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "purchase not found", Code: ErrCodeNotFound})
		return
	}
	if err != nil {
		writeInternal(c, err)
		return
	}
	if purchase.Status != models.PurchasePending {
		c.JSON(http.StatusConflict, ErrorResponse{Error: "purchase not pending", Code: ErrCodeConflict})
		return
	}

	ev, err := models.NewEvent(models.EventPurchaseConfirmed, "api", gin.H{
		"purchase_id": purchase.ID,
		"user_id":     purchase.UserID,
		"promo_id":    purchase.PromoID,
		"partner_id":  purchase.PartnerID,
		"amount":      purchase.Amount,
	})
	if err != nil {
		writeInternal(c, err)
		return
	}
	if err := h.d.Producer.Publish(h.d.Cfg.Kafka.TopicPurchases, strconv.FormatInt(purchase.ID, 10), ev); err != nil {
		slog.Error("publish purchase confirmed", "err", err)
	}
	c.JSON(http.StatusAccepted, SuccessResponse{Data: gin.H{"purchase_id": purchase.ID, "queued": true}})
}

func (h *purchaseHandlers) list(c *gin.Context) {
	uid, ok := auth.MustUserID(c)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	list, err := h.d.Purchases.ListByUser(c.Request.Context(), uid, limit, offset)
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, PaginatedResponse{Data: list, Limit: limit, Offset: offset, HasMore: len(list) == limit})
}
