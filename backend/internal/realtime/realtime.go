// Package realtime confere os eventos de tempo real que chegam de outras
// instâncias pelo Redis antes de irem para o painel.
//
// Cada tipo de evento é refeito no mesmo tipo Go que o backend publica
// (Position, Event, Command…): campo com tipo errado — um rumo em texto, por
// exemplo — derruba o evento, e o que não pertence ao tipo é descartado. Em
// seguida os valores que o painel usa para desenhar o mapa são conferidos
// contra as faixas possíveis.
package realtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"

	"github.com/google/uuid"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/commands"
	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/devices"
	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/events"
	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/tracking"
	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/websocket"
)

// ErrUnknownType: tipo de evento que nenhuma instância publica.
var ErrUnknownType = errors.New("tipo de evento desconhecido")

// DeviceStatus é o "data" de device.online/offline/stale.
type DeviceStatus struct {
	DeviceID uuid.UUID       `json:"deviceId"`
	Status   string          `json:"status,omitempty"`
	Previous string          `json:"previous,omitempty"`
	State    *tracking.State `json:"state,omitempty"`
}

// EngineStatus é o "data" de engine.status.changed.
type EngineStatus struct {
	DeviceID uuid.UUID `json:"deviceId"`
	RelayOn  *bool     `json:"relayOn"`
}

// Decode satisfaz websocket.PayloadDecoder.
func Decode(eventType string, raw json.RawMessage) (any, error) {
	switch eventType {
	case websocket.TypePositionUpdated:
		var p tracking.Position
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return &p, checkPosition(&p)

	case websocket.TypeDeviceOnline, websocket.TypeDeviceOffline, websocket.TypeDeviceStale:
		var s DeviceStatus
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		return &s, checkDeviceStatus(&s)

	case websocket.TypeEngineStatusChanged:
		var e EngineStatus
		if err := json.Unmarshal(raw, &e); err != nil {
			return nil, err
		}
		if e.DeviceID == uuid.Nil || e.RelayOn == nil {
			return nil, errors.New("deviceId e relayOn são obrigatórios")
		}
		return &e, nil

	case websocket.TypeVehicleEvent:
		var e events.Event
		if err := json.Unmarshal(raw, &e); err != nil {
			return nil, err
		}
		return &e, checkEvent(&e)

	case websocket.TypeCommandSent, websocket.TypeCommandAcknowledged, websocket.TypeCommandFailed:
		var c commands.Command
		if err := json.Unmarshal(raw, &c); err != nil {
			return nil, err
		}
		return &c, checkCommand(&c)
	}
	return nil, ErrUnknownType
}

// ValidHeading diz se o rumo está na faixa que o mapa desenha (0 a 360 graus).
func ValidHeading(h float64) bool {
	return finite(h) && h >= 0 && h <= 360
}

func finite(f float64) bool { return !math.IsNaN(f) && !math.IsInf(f, 0) }

func finitePtr(f *float64) bool { return f == nil || finite(*f) }

// maxSpeedKmh é folga sobre qualquer veículo terrestre; acima disso é lixo.
const maxSpeedKmh = 1000

var (
	protocolPattern  = regexp.MustCompile(`^[a-z0-9_-]{0,32}$`)
	eventTypePattern = regexp.MustCompile(`^[A-Z0-9_]{1,64}$`)
)

func checkCoordinates(lat, lon float64) error {
	if !finite(lat) || !finite(lon) || lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return fmt.Errorf("coordenada fora da faixa: %v, %v", lat, lon)
	}
	return nil
}

func checkPosition(p *tracking.Position) error {
	if p.DeviceID == uuid.Nil {
		return errors.New("posição sem deviceId")
	}
	if err := checkCoordinates(p.Latitude, p.Longitude); err != nil {
		return err
	}
	if !finite(p.SpeedKmh) || p.SpeedKmh < 0 || p.SpeedKmh > maxSpeedKmh {
		return fmt.Errorf("velocidade fora da faixa: %v", p.SpeedKmh)
	}
	if p.Heading != nil && !ValidHeading(*p.Heading) {
		return fmt.Errorf("rumo fora da faixa: %v", *p.Heading)
	}
	if !finitePtr(p.Altitude) || !finitePtr(p.HDOP) || !finitePtr(p.BatteryVoltage) {
		return errors.New("valor numérico inválido")
	}
	switch p.Source {
	case "", tracking.SourceGPS, tracking.SourceHeartbeat, tracking.SourceLBS:
	default:
		return fmt.Errorf("origem desconhecida: %q", p.Source)
	}
	if !protocolPattern.MatchString(p.Protocol) {
		return errors.New("protocolo inválido")
	}
	return nil
}

func validDeviceStatus(s string) bool {
	switch s {
	case "", devices.StatusOnline, devices.StatusStale, devices.StatusOffline:
		return true
	}
	return false
}

func checkDeviceStatus(s *DeviceStatus) error {
	if s.DeviceID == uuid.Nil {
		return errors.New("status sem deviceId")
	}
	if !validDeviceStatus(s.Status) || !validDeviceStatus(s.Previous) {
		return errors.New("status desconhecido")
	}
	if s.State != nil && !finitePtr(s.State.BatteryVoltage) {
		return errors.New("valor numérico inválido")
	}
	return nil
}

func checkEvent(e *events.Event) error {
	if e.DeviceID == uuid.Nil || !eventTypePattern.MatchString(e.Type) {
		return errors.New("evento sem aparelho ou com tipo inválido")
	}
	if (e.Latitude == nil) != (e.Longitude == nil) {
		return errors.New("coordenada incompleta")
	}
	if e.Latitude != nil {
		if err := checkCoordinates(*e.Latitude, *e.Longitude); err != nil {
			return err
		}
	}
	if !finitePtr(e.SpeedKmh) {
		return errors.New("valor numérico inválido")
	}
	return nil
}

func checkCommand(c *commands.Command) error {
	if c.ID == uuid.Nil || c.DeviceID == uuid.Nil {
		return errors.New("comando sem id ou aparelho")
	}
	switch c.Status {
	case commands.StatusPending, commands.StatusSending, commands.StatusSent, commands.StatusAcknowledged,
		commands.StatusFailed, commands.StatusTimeout, commands.StatusRejected:
		return nil
	}
	return fmt.Errorf("status de comando desconhecido: %q", c.Status)
}
