// Package websocket entrega os eventos em tempo real ao painel.
package websocket

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Tipos de evento publicados no canal (§18).
const (
	TypePositionUpdated     = "position.updated"
	TypeDeviceOnline        = "device.online"
	TypeDeviceOffline       = "device.offline"
	TypeDeviceStale         = "device.stale"
	TypeVehicleEvent        = "vehicle.event"
	TypeCommandSent         = "command.sent"
	TypeCommandAcknowledged = "command.acknowledged"
	TypeCommandFailed       = "command.failed"
	TypeEngineStatusChanged = "engine.status.changed"
)

// Message é o envelope entregue ao navegador.
type Message struct {
	Type      string     `json:"type"`
	VehicleID *uuid.UUID `json:"vehicleId,omitempty"`
	DeviceID  *uuid.UUID `json:"deviceId,omitempty"`
	Timestamp time.Time  `json:"timestamp"`
	Data      any        `json:"data,omitempty"`

	// origin identifica a instância que originou a mensagem, evitando que ela
	// volte duplicada pelo Redis.
	origin string
}

// Filter decide se uma mensagem vai para um cliente. Nulo entrega tudo (a
// equipe da central); o cliente final recebe só o que é dos veículos dele.
// Roda a cada mensagem, dentro do broadcast: precisa ser rápido.
type Filter func(Message) bool

type client struct {
	send   chan []byte
	id     uuid.UUID
	filter Filter
}

// Hub mantém os clientes conectados e distribui as mensagens.
//
// Nenhum envio bloqueia a ingestão: cliente lento perde mensagem e, se o
// buffer estourar, é desconectado.
type Hub struct {
	mu      sync.RWMutex
	clients map[*client]struct{}

	log      *slog.Logger
	originID string

	// forward recebe cada mensagem publicada localmente, para replicação
	// entre instâncias (Redis). Nulo quando o Redis está desligado.
	forward func(Message)

	onCount func(int)
}

func NewHub(log *slog.Logger, onCount func(int)) *Hub {
	return &Hub{
		clients:  make(map[*client]struct{}),
		log:      log.With("component", "websocket"),
		originID: uuid.NewString(),
		onCount:  onCount,
	}
}

// SetForwarder liga a replicação entre instâncias.
func (h *Hub) SetForwarder(fn func(Message)) { h.forward = fn }

// Publish satisfaz a interface esperada pelos serviços de domínio.
func (h *Hub) Publish(eventType string, data any) {
	h.PublishFor(eventType, nil, nil, data)
}

// PublishFor envia uma mensagem já associada a um veículo/dispositivo.
func (h *Hub) PublishFor(eventType string, vehicleID, deviceID *uuid.UUID, data any) {
	msg := Message{
		Type:      eventType,
		VehicleID: vehicleID,
		DeviceID:  deviceID,
		Timestamp: time.Now().UTC(),
		Data:      data,
		origin:    h.originID,
	}
	h.broadcast(msg)
	if h.forward != nil {
		h.forward(msg)
	}
}

// Deliver entrega uma mensagem vinda de outra instância.
func (h *Hub) Deliver(msg Message) {
	if msg.origin == h.originID {
		return // é a nossa própria mensagem voltando pelo Redis
	}
	h.broadcast(msg)
}

func (h *Hub) broadcast(msg Message) {
	payload, err := json.Marshal(msg)
	if err != nil {
		h.log.Error("falha ao serializar mensagem", "type", msg.Type, "err", err)
		return
	}

	var stale []*client

	h.mu.RLock()
	for c := range h.clients {
		if c.filter != nil && !c.filter(msg) {
			continue
		}
		select {
		case c.send <- payload:
		default:
			stale = append(stale, c)
		}
	}
	h.mu.RUnlock()

	for _, c := range stale {
		h.log.Warn("cliente WebSocket lento, desconectando", "client", c.id)
		h.remove(c)
	}
}

func (h *Hub) add(c *client) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	total := len(h.clients)
	h.mu.Unlock()
	h.notify(total)
}

func (h *Hub) remove(c *client) {
	h.mu.Lock()
	_, ok := h.clients[c]
	if ok {
		delete(h.clients, c)
		close(c.send)
	}
	total := len(h.clients)
	h.mu.Unlock()
	if ok {
		h.notify(total)
	}
}

func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *Hub) notify(total int) {
	if h.onCount != nil {
		h.onCount(total)
	}
}
