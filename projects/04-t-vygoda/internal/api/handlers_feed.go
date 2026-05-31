package api

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/andrey/t-vygoda/internal/auth"
	"github.com/andrey/t-vygoda/internal/models"
	"github.com/andrey/t-vygoda/internal/repo"
)

type feedHandlers struct {
	d *Deps
}

func (h *feedHandlers) personalized(c *gin.Context) {
	uid, ok := auth.MustUserID(c)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	ctx, cancel := context.WithTimeout(c.Request.Context(), 800*time.Millisecond)
	defer cancel()

	promoIDs := h.promoIDsFromCache(ctx, uid)
	if len(promoIDs) == 0 {
		live := h.d.Recommender.GetRecommendations(ctx, uid)
		for _, r := range live {
			promoIDs = append(promoIDs, r.PromoID)
		}
		if len(live) > 0 {
			if err := h.d.RecsCache.SetForUser(c.Request.Context(), uid, live, time.Hour); err != nil {
				slog.Warn("recs cache write", "err", err, "user_id", uid)
			}
		}
	}

	sort := models.FeedSortRecommended
	if len(promoIDs) == 0 {
		sort = models.FeedSortPopular
	}

	posts, err := h.d.Posts.GetFeed(c.Request.Context(), repo.FeedQuery{
		ViewerID: uid,
		Sort:     sort,
		PromoIDs: promoIDs,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		writeInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, PaginatedResponse{Data: posts, Limit: limit, Offset: offset, HasMore: len(posts) == limit})
}

func (h *feedHandlers) promoIDsFromCache(ctx context.Context, userID int64) []int64 {
	cached, err := h.d.RecsCache.GetForUser(ctx, userID)
	if err != nil || len(cached) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(cached))
	for _, r := range cached {
		ids = append(ids, r.PromoID)
	}
	return ids
}
