package tracking

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/commands"
	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/database"
)

// SnapshotProvider entrega ao módulo de comandos o dado em que a regra de
// segurança do corte de motor se apoia (§14).
//
// É um tipo próprio, e não um método do Ingestor, porque o serviço de comandos
// é construído antes da pipeline de ingestão — e porque a regra de segurança
// só precisa do repositório de posições.
type SnapshotProvider struct {
	positions *Repository
}

func NewSnapshotProvider(positions *Repository) *SnapshotProvider {
	return &SnapshotProvider{positions: positions}
}

func (p *SnapshotProvider) Snapshot(ctx context.Context, deviceID uuid.UUID) (commands.Snapshot, error) {
	position, err := p.positions.LatestWithLocation(ctx, deviceID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return commands.Snapshot{}, nil
		}
		return commands.Snapshot{}, err
	}

	return commands.Snapshot{
		HasPosition: true,
		SpeedKmh:    position.SpeedKmh,
		// Usa received_at, e não gps_timestamp: o que importa para autorizar o
		// corte é há quanto tempo o servidor teve notícia do veículo. Um pacote
		// atrasado, vindo do buffer offline do aparelho, não pode passar por
		// informação fresca (§35).
		Timestamp: position.ReceivedAt,
	}, nil
}
