package websocket

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

var testSecret = []byte("segredo-de-teste-com-mais-de-32-bytes!!")

type testPosition struct {
	DeviceID uuid.UUID `json:"deviceId"`
	Heading  *float64  `json:"heading"`
}

// decodeTestPosition faz o papel do realtime.Decode: tipa o "data".
func decodeTestPosition(eventType string, raw json.RawMessage) (any, error) {
	if eventType != TypePositionUpdated {
		return nil, errors.New("tipo desconhecido")
	}
	var p testPosition
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func testMessage(now time.Time) Message {
	device := uuid.New()
	heading := 90.0
	return Message{
		Type: TypePositionUpdated, DeviceID: &device, Timestamp: now,
		Data: &testPosition{DeviceID: device, Heading: &heading}, origin: "instancia-a",
	}
}

func TestWireRoundTrip(t *testing.T) {
	codec := newWireCodec(testSecret, decodeTestPosition)
	msg := testMessage(time.Now().UTC())

	payload, err := codec.encode(msg)
	if err != nil {
		t.Fatal(err)
	}
	got, err := codec.decodeWire(payload)
	if err != nil {
		t.Fatalf("mensagem legítima recusada: %v", err)
	}
	if got.Type != msg.Type || *got.DeviceID != *msg.DeviceID || got.origin != "instancia-a" {
		t.Fatalf("envelope diferente: %+v", got)
	}
	p, ok := got.Data.(*testPosition)
	if !ok || *p.Heading != 90 {
		t.Fatalf("data não foi refeito no tipo: %#v", got.Data)
	}
}

func TestWireRejectsForgedMessages(t *testing.T) {
	codec := newWireCodec(testSecret, decodeTestPosition)
	now := time.Now().UTC()
	legit, err := codec.encode(testMessage(now))
	if err != nil {
		t.Fatal(err)
	}

	// O que alguém com acesso ao Redis publicaria: o formato antigo, sem assinatura.
	unsigned := `{"type":"position.updated","timestamp":"` + now.Format(time.RFC3339Nano) +
		`","origin":"x","data":{"deviceId":"` + uuid.NewString() + `","heading":"0) scale(9)\" onload=\"alert(1)"}}`

	// Mensagem assinada de verdade, com o "data" trocado depois.
	var signed signedMessage
	if err := json.Unmarshal(legit, &signed); err != nil {
		t.Fatal(err)
	}
	signed.Msg = json.RawMessage(strings.Replace(string(signed.Msg), `"heading":90`, `"heading":"<svg onload=alert(1)>"`, 1))
	tampered, _ := json.Marshal(signed)

	// Assinada com outra chave (outro JWT_SECRET).
	other := newWireCodec([]byte("outro-segredo-qualquer-com-32-bytes-ou-mais"), decodeTestPosition)
	wrongKey, _ := other.encode(testMessage(now))

	for name, payload := range map[string]string{
		"sem assinatura":   unsigned,
		"data adulterado":  string(tampered),
		"chave errada":     string(wrongKey),
		"assinatura vazia": `{"msg":{"type":"position.updated"},"sig":""}`,
		"json quebrado":    `{"msg":`,
		"mensagem ausente": `{"sig":"AAAA"}`,
		"texto qualquer":   `PWNED`,
	} {
		if _, err := codec.decodeWire([]byte(payload)); !errors.Is(err, errUnsigned) {
			t.Errorf("%s: esperava errUnsigned, veio %v", name, err)
		}
	}
}

func TestWireRejectsStaleAndFutureMessages(t *testing.T) {
	codec := newWireCodec(testSecret, decodeTestPosition)
	now := time.Now().UTC()
	codec.now = func() time.Time { return now }

	for name, at := range map[string]time.Time{
		"reenviada horas depois": now.Add(-3 * time.Hour),
		"do futuro":              now.Add(10 * time.Minute),
	} {
		payload, err := codec.encode(testMessage(at))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := codec.decodeWire(payload); !errors.Is(err, errExpired) {
			t.Errorf("%s: esperava errExpired, veio %v", name, err)
		}
	}

	payload, _ := codec.encode(testMessage(now.Add(-30 * time.Second)))
	if _, err := codec.decodeWire(payload); err != nil {
		t.Errorf("atraso normal recusado: %v", err)
	}
}

func TestWireRejectsSignedButMistypedData(t *testing.T) {
	codec := newWireCodec(testSecret, decodeTestPosition)
	msg := testMessage(time.Now().UTC())
	msg.Data = map[string]any{"deviceId": msg.DeviceID.String(), "heading": `0" onload="alert(1)`}

	payload, err := codec.encode(msg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := codec.decodeWire(payload); err == nil {
		t.Fatal("rumo em texto passou pela validação")
	}

	msg.Type = "tipo.inventado"
	payload, _ = codec.encode(msg)
	if _, err := codec.decodeWire(payload); err == nil {
		t.Fatal("tipo desconhecido passou pela validação")
	}
}
