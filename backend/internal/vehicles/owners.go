package vehicles

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/database"
)

// OwnerIndex responde, em memória, de qual cliente é cada veículo e cada
// rastreador. O WebSocket consulta o índice a cada mensagem para entregar ao
// cliente só o que é dele, então a consulta não pode ir ao banco.
//
// O índice se recarrega sozinho a cada intervalo (o que cobre alterações
// feitas em outras instâncias) e na hora quando esta instância altera um
// veículo (Refresh).
type OwnerIndex struct {
	db  *database.DB
	log *slog.Logger

	mu        sync.RWMutex
	byVehicle map[uuid.UUID]uuid.UUID
	byDevice  map[uuid.UUID]uuid.UUID
}

func NewOwnerIndex(db *database.DB, log *slog.Logger) *OwnerIndex {
	return &OwnerIndex{
		db:        db,
		log:       log.With("component", "vehicle-owners"),
		byVehicle: map[uuid.UUID]uuid.UUID{},
		byDevice:  map[uuid.UUID]uuid.UUID{},
	}
}

// Refresh relê os donos do banco.
func (o *OwnerIndex) Refresh(ctx context.Context) error {
	rows, err := o.db.Query(ctx, `SELECT id, device_id, owner_id FROM vehicles WHERE owner_id IS NOT NULL`)
	if err != nil {
		return database.MapError(err)
	}
	defer rows.Close()

	byVehicle := map[uuid.UUID]uuid.UUID{}
	byDevice := map[uuid.UUID]uuid.UUID{}
	for rows.Next() {
		var vehicleID, ownerID uuid.UUID
		var deviceID *uuid.UUID
		if err := rows.Scan(&vehicleID, &deviceID, &ownerID); err != nil {
			return err
		}
		byVehicle[vehicleID] = ownerID
		if deviceID != nil {
			byDevice[*deviceID] = ownerID
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	o.mu.Lock()
	o.byVehicle, o.byDevice = byVehicle, byDevice
	o.mu.Unlock()
	return nil
}

// Run recarrega o índice periodicamente até o contexto acabar.
func (o *OwnerIndex) Run(ctx context.Context, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := o.Refresh(ctx); err != nil && ctx.Err() == nil {
				o.log.Warn("falha ao recarregar os donos dos veículos", "err", err)
			}
		}
	}
}

// Owns diz se a mensagem sobre este veículo/rastreador pertence ao cliente.
// Sem nenhum dos dois identificadores, a resposta é não.
func (o *OwnerIndex) Owns(customerID uuid.UUID, vehicleID, deviceID *uuid.UUID) bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	if vehicleID != nil {
		owner, ok := o.byVehicle[*vehicleID]
		return ok && owner == customerID
	}
	if deviceID != nil {
		owner, ok := o.byDevice[*deviceID]
		return ok && owner == customerID
	}
	return false
}
