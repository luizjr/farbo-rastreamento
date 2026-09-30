package websocket

import (
	"encoding/json"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
)

func newTestHub() *Hub {
	return NewHub(slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
}

func drain(c *client) []Message {
	var out []Message
	for {
		select {
		case payload := <-c.send:
			var msg Message
			_ = json.Unmarshal(payload, &msg)
			out = append(out, msg)
		default:
			return out
		}
	}
}

func TestBroadcastRespectsClientFilter(t *testing.T) {
	hub := newTestHub()
	mine, theirs := uuid.New(), uuid.New()

	staff := &client{send: make(chan []byte, 8), id: uuid.New()}
	customer := &client{
		send: make(chan []byte, 8), id: uuid.New(),
		filter: func(msg Message) bool { return msg.VehicleID != nil && *msg.VehicleID == mine },
	}
	hub.add(staff)
	hub.add(customer)

	hub.PublishFor(TypePositionUpdated, &mine, nil, map[string]int{"n": 1})
	hub.PublishFor(TypePositionUpdated, &theirs, nil, map[string]int{"n": 2})
	hub.Publish(TypeDeviceOffline, nil)

	if got := drain(staff); len(got) != 3 {
		t.Fatalf("a equipe deveria receber as 3 mensagens, recebeu %d", len(got))
	}
	got := drain(customer)
	if len(got) != 1 || got[0].VehicleID == nil || *got[0].VehicleID != mine {
		t.Fatalf("o cliente deveria receber só a mensagem do próprio veículo, recebeu %+v", got)
	}
}
