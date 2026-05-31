package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/andrey/t-vygoda/internal/auth"
	"github.com/andrey/t-vygoda/internal/models"
	"github.com/andrey/t-vygoda/internal/repo"
)

type postHandlers struct {
	d *Deps
}

func (h *postHandlers) create(c *gin.Context) {
	uid, ok := auth.MustUserID(c)
	if !ok {
		return
	}
	var in models.CreatePostInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: ErrCodeBadRequest})
		return
	}
	post, err := h.d.Posts.Create(c.Request.Context(), repo.CreatePost{
		UserID:      uid,
		Title:       in.Title,
		Description: in.Description,
		ImageURL:    in.ImageURL,
		PriceBefore: in.PriceBefore,
		PriceAfter:  in.PriceAfter,
		PromoID:     in.PromoID,
		CategoryID:  in.CategoryID,
	})
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusCreated, SuccessResponse{Data: post})
}

func (h *postHandlers) get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "bad id", Code: ErrCodeBadRequest})
		return
	}
	viewer := auth.UserID(c)
	post, err := h.d.Posts.GetByID(c.Request.Context(), id, viewer)
	if errors.Is(err, repo.ErrNotFound) {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "post not found", Code: ErrCodeNotFound})
		return
	}
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, SuccessResponse{Data: post})
}

func (h *postHandlers) list(c *gin.Context) {
	viewer := auth.UserID(c)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	sort := models.FeedSort(c.DefaultQuery("sort_by", "recent"))
	if !sort.Valid() {
		sort = models.FeedSortRecent
	}

	var categoryID *int64
	if raw := c.Query("category"); raw != "" {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil {
			categoryID = &id
		}
	}

	q := repo.FeedQuery{
		ViewerID:   viewer,
		CategoryID: categoryID,
		Sort:       sort,
		Limit:      limit,
		Offset:     offset,
	}

	if sort == models.FeedSortRecommended {
		ids := h.recommendedPromoIDs(c, viewer)
		q.PromoIDs = ids
	}

	posts, err := h.d.Posts.GetFeed(c.Request.Context(), q)
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, PaginatedResponse{Data: posts, Limit: limit, Offset: offset, HasMore: len(posts) == limit})
}

func (h *postHandlers) listByUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "bad id", Code: ErrCodeBadRequest})
		return
	}
	viewer := auth.UserID(c)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	posts, err := h.d.Posts.GetByUser(c.Request.Context(), id, viewer, limit, offset)
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, PaginatedResponse{Data: posts, Limit: limit, Offset: offset, HasMore: len(posts) == limit})
}

func (h *postHandlers) like(c *gin.Context) {
	uid, ok := auth.MustUserID(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "bad id", Code: ErrCodeBadRequest})
		return
	}
	if err := h.d.Posts.Like(c.Request.Context(), id, uid); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "post not found", Code: ErrCodeNotFound})
			return
		}
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, SuccessResponse{Data: gin.H{"liked": true}})
}

func (h *postHandlers) unlike(c *gin.Context) {
	uid, ok := auth.MustUserID(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "bad id", Code: ErrCodeBadRequest})
		return
	}
	if err := h.d.Posts.Unlike(c.Request.Context(), id, uid); err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, SuccessResponse{Data: gin.H{"liked": false}})
}

func (h *postHandlers) recommendedPromoIDs(c *gin.Context, userID int64) []int64 {
	if userID == 0 {
		return nil
	}
	if cached, err := h.d.RecsCache.GetForUser(c.Request.Context(), userID); err == nil && len(cached) > 0 {
		ids := make([]int64, 0, len(cached))
		for _, r := range cached {
			ids = append(ids, r.PromoID)
		}
		return ids
	}
	stored, err := h.d.Recommendations.ForUser(c.Request.Context(), userID, 50)
	if err != nil {
		return nil
	}
	ids := make([]int64, 0, len(stored))
	for _, r := range stored {
		ids = append(ids, r.PromoID)
	}
	return ids
}
