package websocket

import (
	"net/http"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 1024
	sendBuffer     = 256
)

// Handler aceita as conexões do painel.
type Handler struct {
	hub            *Hub
	allowedOrigins []string

	// Scope, quando definido, devolve o filtro de mensagens de quem está
	// conectando (ver Filter). É preenchido pela API, que conhece o usuário.
	Scope func(r *http.Request) Filter
}

func NewHandler(hub *Hub, allowedOrigins []string) *Handler {
	return &Handler{hub: hub, allowedOrigins: allowedOrigins}
}

func (h *Handler) upgrader() *websocket.Upgrader {
	return &websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 4096,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true // cliente não-navegador (ex.: teste automatizado)
			}
			return slices.Contains(h.allowedOrigins, origin)
		},
	}
}

// ServeHTTP promove a requisição a WebSocket. A autenticação acontece antes,
// no middleware da rota.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader().Upgrade(w, r, nil)
	if err != nil {
		return // o Upgrade já respondeu ao cliente
	}

	c := &client{send: make(chan []byte, sendBuffer), id: uuid.New()}
	if h.Scope != nil {
		c.filter = h.Scope(r)
	}
	h.hub.add(c)

	go h.writePump(conn, c)
	go h.readPump(conn, c)
}

// readPump só existe para processar pongs e detectar desconexão: o painel não
// envia comandos por WebSocket (eles passam pela API REST, que audita).
func (h *Handler) readPump(conn *websocket.Conn, c *client) {
	defer func() {
		h.hub.remove(c)
		_ = conn.Close()
	}()

	conn.SetReadLimit(maxMessageSize)
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (h *Handler) writePump(conn *websocket.Conn, c *client) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = conn.Close()
	}()

	for {
		select {
		case payload, ok := <-c.send:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				return
			}
		case <-ticker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
