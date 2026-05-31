package api

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/andrey/t-vygoda/internal/auth"
	"github.com/andrey/t-vygoda/internal/repo"
)

type authHandlers struct {
	d *Deps
}

type registerInput struct {
	Phone        string `json:"phone" binding:"required,min=10,max=20"`
	Name         string `json:"name" binding:"required,min=1,max=200"`
	ReferralCode string `json:"referral_code"`
}

type loginInput struct {
	Phone string `json:"phone" binding:"required"`
	Code  string `json:"code" binding:"required"`
}

type refreshInput struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func (h *authHandlers) register(c *gin.Context) {
	var in registerInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: ErrCodeBadRequest})
		return
	}
	ctx := c.Request.Context()

	if existing, err := h.d.Users.GetByPhone(ctx, in.Phone); err == nil && existing != nil {
		c.JSON(http.StatusConflict, ErrorResponse{Error: "phone already used", Code: ErrCodeConflict})
		return
	}

	var referrerID *int64
	if in.ReferralCode != "" {
		ref, err := h.d.Users.GetByReferralCode(ctx, in.ReferralCode)
		if err == nil {
			referrerID = &ref.ID
		} else if !errors.Is(err, repo.ErrNotFound) {
			writeInternal(c, err)
			return
		}
	}

	code, err := h.d.Users.GenerateUniqueReferralCode(ctx)
	if err != nil {
		writeInternal(c, err)
		return
	}

	user, err := h.d.Users.Create(ctx, repo.CreateUserInput{
		Phone:        in.Phone,
		Name:         in.Name,
		ReferralCode: code,
		ReferredBy:   referrerID,
	})
	if err != nil {
		if errors.Is(err, repo.ErrDuplicate) {
			c.JSON(http.StatusConflict, ErrorResponse{Error: "user exists", Code: ErrCodeConflict})
			return
		}
		writeInternal(c, err)
		return
	}

	if referrerID != nil {
		if err := h.d.Referrals.BuildChainFor(ctx, user.ID, *referrerID); err != nil {
			slog.Error("build referral chain", "err", err, "user_id", user.ID)
		}
	}

	pair, err := h.d.Tokenizer.Issue(user.ID, user.Phone)
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusCreated, SuccessResponse{Data: gin.H{
		"user":   user,
		"tokens": pair,
	}})
}

func (h *authHandlers) login(c *gin.Context) {
	var in loginInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: ErrCodeBadRequest})
		return
	}
	if in.Code != "0000" {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "invalid otp code", Code: ErrCodeUnauthorized})
		return
	}
	user, err := h.d.Users.GetByPhone(c.Request.Context(), in.Phone)
	if errors.Is(err, repo.ErrNotFound) {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "user not found", Code: ErrCodeNotFound})
		return
	}
	if err != nil {
		writeInternal(c, err)
		return
	}
	pair, err := h.d.Tokenizer.Issue(user.ID, user.Phone)
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, SuccessResponse{Data: gin.H{"user": user, "tokens": pair}})
}

func (h *authHandlers) refresh(c *gin.Context) {
	var in refreshInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: ErrCodeBadRequest})
		return
	}
	claims, err := h.d.Tokenizer.Parse(in.RefreshToken)
	if err != nil || claims.Kind != auth.TokenRefresh {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "invalid refresh", Code: ErrCodeUnauthorized})
		return
	}
	pair, err := h.d.Tokenizer.Issue(claims.UserID, claims.Phone)
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, SuccessResponse{Data: pair})
}

func writeInternal(c *gin.Context, err error) {
	slog.Error("handler", "path", c.FullPath(), "err", err)
	c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal error", Code: ErrCodeInternal})
}
