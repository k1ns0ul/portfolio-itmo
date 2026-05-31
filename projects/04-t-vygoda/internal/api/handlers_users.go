package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/andrey/t-vygoda/internal/auth"
	"github.com/andrey/t-vygoda/internal/repo"
)

type userHandlers struct {
	d *Deps
}

type updateMeInput struct {
	Name      string  `json:"name" binding:"required,min=1,max=200"`
	Email     *string `json:"email"`
	AvatarURL *string `json:"avatar_url"`
}

func (h *userHandlers) me(c *gin.Context) {
	uid, ok := auth.MustUserID(c)
	if !ok {
		return
	}
	u, err := h.d.Users.GetByID(c.Request.Context(), uid)
	if err != nil {
		writeInternal(c, err)
		return
	}
	stats, err := h.d.Users.Stats(c.Request.Context(), uid)
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, SuccessResponse{Data: gin.H{"user": u, "stats": stats}})
}

func (h *userHandlers) updateMe(c *gin.Context) {
	uid, ok := auth.MustUserID(c)
	if !ok {
		return
	}
	var in updateMeInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: ErrCodeBadRequest})
		return
	}
	u, err := h.d.Users.Update(c.Request.Context(), uid, in.Name, in.Email, in.AvatarURL)
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, SuccessResponse{Data: u})
}

func (h *userHandlers) get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "bad id", Code: ErrCodeBadRequest})
		return
	}
	u, err := h.d.Users.GetByID(c.Request.Context(), id)
	if errors.Is(err, repo.ErrNotFound) {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "user not found", Code: ErrCodeNotFound})
		return
	}
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, SuccessResponse{Data: u.Public()})
}

func (h *userHandlers) referrals(c *gin.Context) {
	uid, ok := auth.MustUserID(c)
	if !ok {
		return
	}
	list, err := h.d.Referrals.ListReferralsByReferrer(c.Request.Context(), uid, nil)
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, SuccessResponse{Data: list})
}

func (h *userHandlers) bonuses(c *gin.Context) {
	uid, ok := auth.MustUserID(c)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	list, err := h.d.Referrals.ListBonusesByUser(c.Request.Context(), uid, limit)
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, SuccessResponse{Data: list})
}

func (h *userHandlers) streak(c *gin.Context) {
	uid, ok := auth.MustUserID(c)
	if !ok {
		return
	}
	s, err := h.d.Streaks.Get(c.Request.Context(), uid)
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, SuccessResponse{Data: s})
}
