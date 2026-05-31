package coordinator

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/andrey/mpc-cluster/internal/session"
	"github.com/gin-gonic/gin"
)

type Handlers struct {
	manager *session.SessionManager
	store   *session.Store
}

func NewHandlers(m *session.SessionManager, store *session.Store) *Handlers {
	return &Handlers{manager: m, store: store}
}

type createSessionReq struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	PartyCount int    `json:"party_count"`
	Threshold  int    `json:"threshold"`
}

type executeReq struct {
	Inputs map[string]string `json:"inputs"`
}

func (h *Handlers) CreateSession(c *gin.Context) {
	var req createSessionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	sess, err := h.manager.Create(c.Request.Context(), session.CreateRequest{
		ID:         req.ID,
		Type:       session.OpType(req.Type),
		PartyCount: req.PartyCount,
		Threshold:  req.Threshold,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, sess)
}

func (h *Handlers) ExecuteSession(c *gin.Context) {
	id := c.Param("id")
	var req executeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
		return
	}
	inputs := make(map[int]string, len(req.Inputs))
	for k, v := range req.Inputs {
		idx, err := strconv.Atoi(k)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "input key must be integer: " + k})
			return
		}
		inputs[idx] = v
	}
	result, err := h.manager.Execute(c.Request.Context(), id, inputs)
	if err != nil {
		if errors.Is(err, session.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
			return
		}
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"session_id": id, "result": result})
}

func (h *Handlers) GetSession(c *gin.Context) {
	sess, err := h.store.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, session.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, sess)
}

func (h *Handlers) ListSessions(c *gin.Context) {
	limit := atoiDefault(c.Query("limit"), 20)
	offset := atoiDefault(c.Query("offset"), 0)
	if limit < 1 || limit > 200 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	list, total, err := h.store.ListAll(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items":  list,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (h *Handlers) DeleteSession(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.Cancel(c.Request.Context(), id); err != nil {
		if errors.Is(err, session.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "cancelled", "session_id": id})
}

func (h *Handlers) GetRounds(c *gin.Context) {
	rounds, err := h.store.ListRounds(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"rounds": rounds})
}

func (h *Handlers) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}
