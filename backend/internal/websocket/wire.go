package websocket

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// O Redis é só o transporte entre as instâncias: não é confiável. Quem
// alcança o canal poderia publicar um evento falso que iria direto para o
// painel de todos. Por isso cada mensagem viaja assinada (HMAC-SHA256 com uma
// chave derivada do JWT_SECRET, que só as instâncias conhecem), com prazo de
// validade, e o "data" é refeito no tipo que o backend publica antes de ir
// para o navegador.

// maxWireSkew é o quanto o horário da mensagem pode divergir do nosso. Uma
// mensagem legítima atravessa o Redis em milissegundos; o prazo largo só
// acomoda relógios desencontrados entre servidores e impede reenviar uma
// mensagem capturada horas depois.
const maxWireSkew = 2 * time.Minute

// PayloadDecoder refaz o "data" de um evento vindo de outra instância no tipo
// original e confere seus valores. Erro descarta o evento.
type PayloadDecoder func(eventType string, data json.RawMessage) (any, error)

// wireMessage carrega também o origin, para a instância não receber de volta
// o que ela mesma publicou.
type wireMessage struct {
	Type      string          `json:"type"`
	VehicleID *uuid.UUID      `json:"vehicleId,omitempty"`
	DeviceID  *uuid.UUID      `json:"deviceId,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data,omitempty"`
	Origin    string          `json:"origin"`
}

// signedMessage é o que vai para o Redis: a mensagem e a assinatura dela.
type signedMessage struct {
	Msg json.RawMessage `json:"msg"`
	Sig []byte          `json:"sig"`
}

// wireCodec assina e confere as mensagens do canal.
type wireCodec struct {
	key    []byte
	decode PayloadDecoder
	now    func() time.Time
}

func newWireCodec(secret []byte, decode PayloadDecoder) *wireCodec {
	return &wireCodec{key: busKey(secret), decode: decode, now: time.Now}
}

// busKey separa a chave do canal da chave dos JWT: uma assinatura de um
// nunca vale pelo outro.
func busKey(secret []byte) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte("farbo:ws-bus:v1"))
	return mac.Sum(nil)
}

func (c *wireCodec) sign(msg []byte) []byte {
	mac := hmac.New(sha256.New, c.key)
	mac.Write(msg)
	return mac.Sum(nil)
}

func (c *wireCodec) encode(msg Message) ([]byte, error) {
	data, err := json.Marshal(msg.Data)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(wireMessage{
		Type: msg.Type, VehicleID: msg.VehicleID, DeviceID: msg.DeviceID,
		Timestamp: msg.Timestamp, Data: data, Origin: msg.origin,
	})
	if err != nil {
		return nil, err
	}
	return json.Marshal(signedMessage{Msg: body, Sig: c.sign(body)})
}

var (
	errUnsigned = errors.New("assinatura ausente ou inválida")
	errExpired  = errors.New("mensagem fora do prazo")
)

func (c *wireCodec) decodeWire(payload []byte) (Message, error) {
	var signed signedMessage
	if err := json.Unmarshal(payload, &signed); err != nil {
		return Message{}, errUnsigned
	}
	if len(signed.Msg) == 0 || !hmac.Equal(signed.Sig, c.sign(signed.Msg)) {
		return Message{}, errUnsigned
	}

	var w wireMessage
	if err := json.Unmarshal(signed.Msg, &w); err != nil {
		return Message{}, err
	}
	if age := c.now().Sub(w.Timestamp); age > maxWireSkew || age < -maxWireSkew {
		return Message{}, errExpired
	}

	// Mesmo assinada, a mensagem só passa com um "data" do tipo esperado: o
	// navegador recebe o que o backend refez, nunca o JSON como chegou.
	var data any
	if len(w.Data) > 0 && string(w.Data) != "null" {
		decoded, err := c.decode(w.Type, w.Data)
		if err != nil {
			return Message{}, fmt.Errorf("evento %q: %w", w.Type, err)
		}
		data = decoded
	}
	return Message{
		Type: w.Type, VehicleID: w.VehicleID, DeviceID: w.DeviceID,
		Timestamp: w.Timestamp, Data: data, origin: w.Origin,
	}, nil
}
