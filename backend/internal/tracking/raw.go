package tracking

import (
	"context"
	"encoding/hex"
	"strings"
	"time"
	"unicode"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/database"
)

// RawPacket é tráfego que não pôde ser interpretado. Guardar isso é o que
// permite descobrir qual variante de protocolo o aparelho realmente fala, em
// vez de deduzir por semelhança (§38).
type RawPacket struct {
	ID int64 `json:"id"`

	RemoteAddr string `json:"remoteAddr"`
	IMEI       string `json:"imei"`
	Protocol   string `json:"protocol"`
	Reason     string `json:"reason"`

	PayloadHex   string `json:"payloadHex"`
	PayloadASCII string `json:"payloadAscii"`
	ByteCount    int    `json:"byteCount"`

	ReceivedAt time.Time `json:"receivedAt"`
}

type RawPacketRepository struct{ db *database.DB }

func NewRawPacketRepository(db *database.DB) *RawPacketRepository {
	return &RawPacketRepository{db: db}
}

func (r *RawPacketRepository) Insert(ctx context.Context, p *RawPacket) error {
	return database.MapError(r.db.QueryRow(ctx, `
		INSERT INTO raw_packets (remote_addr, imei, protocol, reason,
			payload_hex, payload_ascii, byte_count)
		VALUES ($1, NULLIF($2, ''), NULLIF($3, ''), $4, $5, $6, $7)
		RETURNING id, received_at`,
		p.RemoteAddr, p.IMEI, p.Protocol, p.Reason,
		p.PayloadHex, p.PayloadASCII, p.ByteCount,
	).Scan(&p.ID, &p.ReceivedAt))
}

func (r *RawPacketRepository) List(ctx context.Context, limit int) ([]*RawPacket, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, remote_addr, COALESCE(imei, ''), COALESCE(protocol, ''), reason,
			payload_hex, payload_ascii, byte_count, received_at
		FROM raw_packets ORDER BY received_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, database.MapError(err)
	}
	defer rows.Close()

	out := []*RawPacket{}
	for rows.Next() {
		var p RawPacket
		if err := rows.Scan(&p.ID, &p.RemoteAddr, &p.IMEI, &p.Protocol, &p.Reason,
			&p.PayloadHex, &p.PayloadASCII, &p.ByteCount, &p.ReceivedAt); err != nil {
			return nil, err
		}
		out = append(out, &p)
	}
	return out, rows.Err()
}

// DeleteOlderThan limpa a captura antiga.
func (r *RawPacketRepository) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	tag, err := r.db.Exec(ctx, `DELETE FROM raw_packets WHERE received_at < $1`, cutoff)
	if err != nil {
		return 0, database.MapError(err)
	}
	return tag.RowsAffected(), nil
}

// ToASCII troca bytes não imprimíveis por ponto, para leitura humana no painel.
func ToASCII(b []byte) string {
	var sb strings.Builder
	sb.Grow(len(b))
	for _, c := range b {
		if c < unicode.MaxASCII && unicode.IsPrint(rune(c)) {
			sb.WriteByte(c)
		} else {
			sb.WriteByte('.')
		}
	}
	return sb.String()
}

// ToHex formata o payload em hexadecimal maiúsculo.
func ToHex(b []byte) string { return strings.ToUpper(hex.EncodeToString(b)) }
