package api

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/andrey/distributed-kv/internal/raft"
	"github.com/andrey/distributed-kv/internal/store"
)

const applyTimeout = 5 * time.Second

type Server struct {
	node      *raft.RaftNode
	store     *store.KVStore
	forwarder *Forwarder
	log       *slog.Logger
	engine    *gin.Engine
}

type Deps struct {
	Node      *raft.RaftNode
	Store     *store.KVStore
	Forwarder *Forwarder
	Metrics   Metrics
	Log       *slog.Logger
}

func NewServer(d Deps) *Server {
	s := &Server{node: d.Node, store: d.Store, forwarder: d.Forwarder, log: d.Log}
	s.engine = s.buildRouter(d.Metrics)
	return s
}

func (s *Server) Handler() http.Handler {
	return s.engine
}

func (s *Server) Execute(ctx context.Context, method, path string, body []byte) (int, []byte) {
	req, err := http.NewRequestWithContext(ctx, method, path, bytes.NewReader(body))
	if err != nil {
		return http.StatusInternalServerError, []byte(err.Error())
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Forwarded-Internal", "1")
	rec := newRecorder()
	s.engine.ServeHTTP(rec, req)
	return rec.status, rec.body.Bytes()
}
