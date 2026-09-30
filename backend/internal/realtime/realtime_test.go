package realtime

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/commands"
	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/events"
	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/tracking"
	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/websocket"
)

func ptr[T any](v T) *T { return &v }

func samplePosition() *tracking.Position {
	return &tracking.Position{
		ID: 42, DeviceID: uuid.New(),
		GPSTimestamp: time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC),
		ReceivedAt:   time.Date(2026, 9, 30, 12, 0, 1, 0, time.UTC),
		Latitude:     -23.5505, Longitude: -46.6333, SpeedKmh: 42.5,
		Heading: ptr(271.5), Altitude: ptr(760.0), GPSValid: ptr(true), Satellites: ptr(9),
		HDOP: ptr(0.9), ACC: ptr(true), BatteryVoltage: ptr(4.1), BatteryPercent: ptr(88),
		GSMLevel: ptr(4), RelayOn: ptr(false), Protocol: "gt06", Source: tracking.SourceGPS,
	}
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Um evento legítimo sai idêntico: nada do que o painel usa se perde.
func TestDecodeKeepsLegitimateEvents(t *testing.T) {
	deviceID := uuid.New()
	cases := map[string]any{
		websocket.TypePositionUpdated:     samplePosition(),
		websocket.TypeDeviceOnline:        &DeviceStatus{DeviceID: deviceID, State: &tracking.State{DeviceID: deviceID, ACC: ptr(true), BatteryVoltage: ptr(12.4)}},
		websocket.TypeDeviceStale:         &DeviceStatus{DeviceID: deviceID, Status: "STALE", Previous: "ONLINE"},
		websocket.TypeEngineStatusChanged: &EngineStatus{DeviceID: deviceID, RelayOn: ptr(true)},
		websocket.TypeVehicleEvent: &events.Event{ID: 7, DeviceID: deviceID, Type: "OVERSPEED",
			Timestamp: time.Now().UTC(), Latitude: ptr(-23.5), Longitude: ptr(-46.6), SpeedKmh: ptr(130.0),
			Metadata: map[string]any{"limit": 110.0}},
		websocket.TypeCommandAcknowledged: &commands.Command{ID: uuid.New(), DeviceID: deviceID,
			Command: "ENGINE_STOP", Status: commands.StatusAcknowledged},
	}
	for eventType, data := range cases {
		original := mustJSON(t, data)
		decoded, err := Decode(eventType, original)
		if err != nil {
			t.Errorf("%s legítimo recusado: %v", eventType, err)
			continue
		}
		if again := mustJSON(t, decoded); string(again) != string(original) {
			t.Errorf("%s mudou ao passar pela validação:\n antes: %s\ndepois: %s", eventType, original, again)
		}
	}

	// O formato em map que o ingestor publica para status também passa.
	raw := mustJSON(t, map[string]any{"deviceId": deviceID, "status": "OFFLINE", "previous": "STALE"})
	if _, err := Decode(websocket.TypeDeviceOffline, raw); err != nil {
		t.Errorf("status em map recusado: %v", err)
	}
}

func positionWith(t *testing.T, field string, value any) json.RawMessage {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(mustJSON(t, samplePosition()), &m); err != nil {
		t.Fatal(err)
	}
	m[field] = value
	return mustJSON(t, m)
}

func TestDecodeRejectsBadHeading(t *testing.T) {
	cases := map[string]json.RawMessage{
		"texto com markup": positionWith(t, "heading", `0 16 16)" onload="alert(1)`),
		"número em texto":  positionWith(t, "heading", "90"),
		"objeto":           positionWith(t, "heading", map[string]any{"toString": "x"}),
		"lista":            positionWith(t, "heading", []any{1, 2}),
		"booleano":         positionWith(t, "heading", true),
		"acima de 360":     positionWith(t, "heading", 361),
		"negativo":         positionWith(t, "heading", -1),
		"infinito":         json.RawMessage(strings.Replace(string(positionWith(t, "heading", 1)), `"heading":1`, `"heading":1e999`, 1)),
	}
	for name, raw := range cases {
		if _, err := Decode(websocket.TypePositionUpdated, raw); err == nil {
			t.Errorf("rumo %s passou pela validação", name)
		}
	}
	for _, ok := range []any{0, 359.9, 360, nil} {
		if _, err := Decode(websocket.TypePositionUpdated, positionWith(t, "heading", ok)); err != nil {
			t.Errorf("rumo %v válido recusado: %v", ok, err)
		}
	}
}

func TestDecodeRejectsOtherBadPositionValues(t *testing.T) {
	cases := map[string]json.RawMessage{
		"velocidade em texto":  positionWith(t, "speedKmh", `3" onload="x`),
		"velocidade negativa":  positionWith(t, "speedKmh", -5),
		"velocidade absurda":   positionWith(t, "speedKmh", 5000),
		"latitude fora":        positionWith(t, "latitude", 91),
		"longitude fora":       positionWith(t, "longitude", -181),
		"latitude em texto":    positionWith(t, "latitude", "-23"),
		"ignição em texto":     positionWith(t, "acc", "true"),
		"aparelho vazio":       positionWith(t, "deviceId", uuid.Nil.String()),
		"aparelho não-uuid":    positionWith(t, "deviceId", `"><img src=x onerror=alert(1)>`),
		"origem inventada":     positionWith(t, "source", "<b>x</b>"),
		"protocolo com markup": positionWith(t, "protocol", "<script>alert(1)</script>"),
		"não é objeto":         json.RawMessage(`"<svg onload=alert(1)>"`),
	}
	for name, raw := range cases {
		if _, err := Decode(websocket.TypePositionUpdated, raw); err == nil {
			t.Errorf("%s passou pela validação", name)
		}
	}
}

// Campo que não pertence ao tipo não chega ao navegador.
func TestDecodeDropsUnknownFields(t *testing.T) {
	raw := positionWith(t, "html", `<img src=x onerror=alert(1)>`)
	decoded, err := Decode(websocket.TypePositionUpdated, raw)
	if err != nil {
		t.Fatal(err)
	}
	if out := string(mustJSON(t, decoded)); strings.Contains(out, "onerror") {
		t.Fatalf("campo desconhecido atravessou: %s", out)
	}
}

func TestDecodeRejectsOtherEvents(t *testing.T) {
	deviceID := uuid.New().String()
	cases := []struct {
		name, eventType string
		raw             string
	}{
		{"tipo desconhecido", "script.run", `{}`},
		{"status inventado", websocket.TypeDeviceOnline, `{"deviceId":"` + deviceID + `","status":"<b>ON</b>"}`},
		{"status sem aparelho", websocket.TypeDeviceStale, `{"status":"STALE"}`},
		{"relé em texto", websocket.TypeEngineStatusChanged, `{"deviceId":"` + deviceID + `","relayOn":"true"}`},
		{"relé ausente", websocket.TypeEngineStatusChanged, `{"deviceId":"` + deviceID + `"}`},
		{"evento com tipo em markup", websocket.TypeVehicleEvent, `{"deviceId":"` + deviceID + `","type":"<svg/onload=1>"}`},
		{"evento com meia coordenada", websocket.TypeVehicleEvent, `{"deviceId":"` + deviceID + `","type":"SOS","latitude":1}`},
		{"comando com status inventado", websocket.TypeCommandFailed, `{"id":"` + uuid.NewString() + `","deviceId":"` + deviceID + `","status":"x"}`},
	}
	for _, c := range cases {
		if _, err := Decode(c.eventType, json.RawMessage(c.raw)); err == nil {
			t.Errorf("%s passou pela validação", c.name)
		}
	}
	if _, err := Decode("script.run", json.RawMessage(`{}`)); !errors.Is(err, ErrUnknownType) {
		t.Errorf("tipo desconhecido deveria ser ErrUnknownType, veio %v", err)
	}
}
