package websocket

import (
	"context"
	"log/slog"

	"github.com/redis/go-redis/v9"
)

// channelName é o canal usado para replicar eventos entre instâncias.
const channelName = "tracker:events"

// Bridge replica as mensagens do hub entre várias instâncias do backend.
// É opcional: sem Redis o hub funciona normalmente em processo único.
//
// As mensagens vão assinadas com uma chave derivada de secret (o JWT_SECRET,
// igual em todas as instâncias) e o "data" de cada uma passa por decode
// antes de chegar ao navegador — ver wire.go.
type Bridge struct {
	client *redis.Client
	hub    *Hub
	codec  *wireCodec
	log    *slog.Logger
}

func NewBridge(client *redis.Client, hub *Hub, secret []byte, decode PayloadDecoder, log *slog.Logger) *Bridge {
	b := &Bridge{client: client, hub: hub, codec: newWireCodec(secret, decode), log: log.With("component", "ws-bridge")}
	hub.SetForwarder(b.forward)
	return b
}

func (b *Bridge) forward(msg Message) {
	payload, err := b.codec.encode(msg)
	if err != nil {
		b.log.Error("falha ao serializar mensagem para o Redis", "err", err)
		return
	}
	// Publicação em segundo plano: a ingestão não pode esperar o Redis.
	go func() {
		if err := b.client.Publish(context.Background(), channelName, payload).Err(); err != nil {
			b.log.Warn("falha ao publicar no Redis", "err", err)
		}
	}()
}

// Run assina o canal e entrega ao hub local até o contexto ser cancelado.
func (b *Bridge) Run(ctx context.Context) {
	sub := b.client.Subscribe(ctx, channelName)
	defer func() { _ = sub.Close() }()

	b.log.Info("replicação de eventos via Redis ativa", "channel", channelName)

	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case raw, ok := <-ch:
			if !ok {
				return
			}
			msg, err := b.codec.decodeWire([]byte(raw.Payload))
			if err != nil {
				// Não loga o conteúdo: pode ser justamente a tentativa de injeção.
				b.log.Warn("mensagem descartada vinda do Redis", "err", err)
				continue
			}
			b.hub.Deliver(msg)
		}
	}
}
