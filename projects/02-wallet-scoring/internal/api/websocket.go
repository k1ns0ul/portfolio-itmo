package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

const (
	pingInterval = 30 * time.Second
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1 << 10,
	WriteBufferSize: 1 << 14,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type Hub struct {
	rdb          *redis.Client
	readTimeout  time.Duration
	writeTimeout time.Duration

	mu      sync.RWMutex
	clients map[*wsClient]struct{}

	broadcast chan []byte
	register  chan *wsClient
	unregister chan *wsClient
}

func NewHub(rdb *redis.Client, readTimeout, writeTimeout time.Duration) *Hub {
	if readTimeout == 0 {
		readTimeout = 60 * time.Second
	}
	if writeTimeout == 0 {
		writeTimeout = 10 * time.Second
	}
	return &Hub{
		rdb:          rdb,
		readTimeout:  readTimeout,
		writeTimeout: writeTimeout,
		clients:      make(map[*wsClient]struct{}),
		broadcast:    make(chan []byte, 1024),
		register:     make(chan *wsClient, 64),
		unregister:   make(chan *wsClient, 64),
	}
}

func (h *Hub) run() {
	go h.consumeRedis(context.Background(), "score-updates")
	go h.consumeRedis(context.Background(), "alerts")

	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			h.clients[c] = struct{}{}
			h.mu.Unlock()
		case c := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
			}
			h.mu.Unlock()
		case msg := <-h.broadcast:
			h.fanout(msg)
		}
	}
}

func (h *Hub) consumeRedis(ctx context.Context, channel string) {
	for {
		sub := h.rdb.Subscribe(ctx, channel)
		ch := sub.Channel()
		for m := range ch {
			env := struct {
				Channel string          `json:"channel"`
				Payload json.RawMessage `json:"payload"`
			}{Channel: channel, Payload: json.RawMessage(m.Payload)}
			b, err := json.Marshal(env)
			if err != nil {
				continue
			}
			select {
			case h.broadcast <- b:
			default:
				slog.Warn("ws broadcast queue full", "channel", channel)
			}
		}
		_ = sub.Close()
		if ctx.Err() != nil {
			return
		}
		time.Sleep(time.Second)
	}
}

func (h *Hub) fanout(msg []byte) {
	var env struct {
		Channel string          `json:"channel"`
		Payload json.RawMessage `json:"payload"`
	}
	_ = json.Unmarshal(msg, &env)
	var hint struct {
		Wallet string `json:"wallet"`
	}
	_ = json.Unmarshal(env.Payload, &hint)

	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		if !c.wants(env.Channel, hint.Wallet) {
			continue
		}
		select {
		case c.send <- msg:
		default:
		}
	}
}

func (h *Hub) handle(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	client := &wsClient{
		hub:           h,
		conn:          conn,
		send:          make(chan []byte, 64),
		subs:          map[string]struct{}{},
		allAlerts:     true,
		allScores:     false,
	}
	h.register <- client
	go client.writeLoop()
	go client.readLoop()
}

type wsClient struct {
	hub       *Hub
	conn      *websocket.Conn
	send      chan []byte
	mu        sync.Mutex
	subs      map[string]struct{}
	allAlerts bool
	allScores bool
}

type clientCommand struct {
	Action  string   `json:"action"`
	Wallets []string `json:"wallets,omitempty"`
	All     bool     `json:"all,omitempty"`
	Channel string   `json:"channel,omitempty"`
}

func (c *wsClient) readLoop() {
	defer func() {
		c.hub.unregister <- c
		_ = c.conn.Close()
	}()
	c.conn.SetReadLimit(4 << 10)
	_ = c.conn.SetReadDeadline(time.Now().Add(c.hub.readTimeout))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(c.hub.readTimeout))
	})
	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		var cmd clientCommand
		if err := json.Unmarshal(raw, &cmd); err != nil {
			continue
		}
		c.applyCommand(cmd)
	}
}

func (c *wsClient) applyCommand(cmd clientCommand) {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch cmd.Action {
	case "subscribe":
		for _, w := range cmd.Wallets {
			c.subs[w] = struct{}{}
		}
		if cmd.All {
			if cmd.Channel == "alerts" {
				c.allAlerts = true
			} else {
				c.allScores = true
			}
		}
	case "unsubscribe":
		for _, w := range cmd.Wallets {
			delete(c.subs, w)
		}
		if cmd.All {
			if cmd.Channel == "alerts" {
				c.allAlerts = false
			} else {
				c.allScores = false
			}
		}
	}
}

func (c *wsClient) wants(channel, wallet string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if channel == "alerts" && c.allAlerts {
		return true
	}
	if channel == "score-updates" && c.allScores {
		return true
	}
	if wallet == "" {
		return false
	}
	_, ok := c.subs[wallet]
	return ok
}

func (c *wsClient) writeLoop() {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()
	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(c.hub.writeTimeout))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(c.hub.writeTimeout))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
