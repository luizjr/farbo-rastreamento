// Package retention decide por quantos dias o histórico (posições e eventos)
// de cada veículo é guardado — 7, 14 ou 30 — e apaga o que venceu.
//
// A regra herda: o veículo pode ter a própria; senão vale a do cliente dono;
// senão o padrão da central (HISTORY_RETENTION_DAYS).
package retention

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/database"
)

// Options são os prazos aceitos.
var Options = []int{7, 14, 30}

// Valid diz se o prazo é um dos aceitos.
func Valid(days int) bool {
	for _, d := range Options {
		if d == days {
			return true
		}
	}
	return false
}

// ErrInvalid: prazo fora de 7, 14 ou 30.
var ErrInvalid = errors.New("o histórico pode ser guardado por 7, 14 ou 30 dias")

const (
	// batchSize limita cada DELETE: transações curtas não travam a ingestão
	// nem incham o WAL de uma vez.
	batchSize = 5000
	// deviceChunk limita quantos aparelhos entram em cada consulta.
	deviceChunk = 500
	// pause entre lotes, para a limpeza dividir o banco com a ingestão.
	pause = 50 * time.Millisecond
)

type Service struct {
	db          *database.DB
	defaultDays int
	log         *slog.Logger
	now         func() time.Time
}

func NewService(db *database.DB, defaultDays int, log *slog.Logger) *Service {
	return &Service{db: db, defaultDays: defaultDays, log: log.With("component", "retention"), now: time.Now}
}

// Default é o padrão da central.
func (s *Service) Default() int { return s.defaultDays }

// Effective resolve o prazo de um veículo.
func (s *Service) Effective(vehicleDays, customerDays *int) int {
	if vehicleDays != nil {
		return *vehicleDays
	}
	if customerDays != nil {
		return *customerDays
	}
	return s.defaultDays
}

// CustomerDays é o prazo configurado no cliente (nil = padrão da central).
func (s *Service) CustomerDays(ctx context.Context, customerID uuid.UUID) (*int, error) {
	var days *int
	err := s.db.QueryRow(ctx, `SELECT history_retention_days FROM users WHERE id = $1`, customerID).Scan(&days)
	return days, database.MapError(err)
}

// CustomersDays traz os clientes com prazo próprio (para montar listas).
func (s *Service) CustomersDays(ctx context.Context) (map[uuid.UUID]int, error) {
	rows, err := s.db.Query(ctx, `SELECT id, history_retention_days FROM users WHERE history_retention_days IS NOT NULL`)
	if err != nil {
		return nil, database.MapError(err)
	}
	defer rows.Close()
	out := map[uuid.UUID]int{}
	for rows.Next() {
		var id uuid.UUID
		var days int
		if err := rows.Scan(&id, &days); err != nil {
			return nil, database.MapError(err)
		}
		out[id] = days
	}
	return out, rows.Err()
}

// ForVehicle resolve o prazo de um veículo lendo o cliente dono.
func (s *Service) ForVehicle(ctx context.Context, vehicleDays *int, ownerID *uuid.UUID) (int, error) {
	if vehicleDays != nil || ownerID == nil {
		return s.Effective(vehicleDays, nil), nil
	}
	customerDays, err := s.CustomerDays(ctx, *ownerID)
	if err != nil {
		return 0, err
	}
	return s.Effective(nil, customerDays), nil
}

func check(days *int) error {
	if days != nil && !Valid(*days) {
		return ErrInvalid
	}
	return nil
}

// SetCustomer define (ou, com nil, volta ao padrão) o prazo do cliente.
func (s *Service) SetCustomer(ctx context.Context, customerID uuid.UUID, days *int) error {
	if err := check(days); err != nil {
		return err
	}
	tag, err := s.db.Exec(ctx, `UPDATE users SET history_retention_days = $2, updated_at = NOW() WHERE id = $1`,
		customerID, days)
	if err != nil {
		return database.MapError(err)
	}
	if tag.RowsAffected() == 0 {
		return database.ErrNotFound
	}
	return nil
}

// SetVehicle define (ou, com nil, volta ao do cliente) o prazo do veículo.
func (s *Service) SetVehicle(ctx context.Context, vehicleID uuid.UUID, days *int) error {
	if err := check(days); err != nil {
		return err
	}
	tag, err := s.db.Exec(ctx, `UPDATE vehicles SET history_retention_days = $2, updated_at = NOW() WHERE id = $1`,
		vehicleID, days)
	if err != nil {
		return database.MapError(err)
	}
	if tag.RowsAffected() == 0 {
		return database.ErrNotFound
	}
	return nil
}

// Result é o que uma limpeza apagou.
type Result struct {
	Positions int64
	Events    int64
}

// Cleanup apaga as posições e os eventos mais velhos que o prazo de cada
// aparelho (pelo veículo em que ele está). Aparelho sem veículo usa o padrão.
func (s *Service) Cleanup(ctx context.Context) (Result, error) {
	rows, err := s.db.Query(ctx, `
		SELECT d.id, COALESCE(v.history_retention_days, u.history_retention_days, $1)::int
		FROM devices d
		LEFT JOIN vehicles v ON v.device_id = d.id
		LEFT JOIN users u ON u.id = v.owner_id`, s.defaultDays)
	if err != nil {
		return Result{}, database.MapError(err)
	}
	groups := map[int][]uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		var days int
		if err := rows.Scan(&id, &days); err != nil {
			rows.Close()
			return Result{}, database.MapError(err)
		}
		groups[days] = append(groups[days], id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Result{}, database.MapError(err)
	}

	days := make([]int, 0, len(groups))
	for d := range groups {
		days = append(days, d)
	}
	sort.Ints(days)

	var res Result
	for _, d := range days {
		cutoff := s.now().Add(-time.Duration(d) * 24 * time.Hour)
		ids := groups[d]
		for start := 0; start < len(ids); start += deviceChunk {
			chunk := ids[start:min(start+deviceChunk, len(ids))]
			n, err := s.deleteBatches(ctx, `
				DELETE FROM positions WHERE id IN (
					SELECT id FROM positions WHERE device_id = ANY($1) AND gps_timestamp < $2 LIMIT $3)`, chunk, cutoff)
			res.Positions += n
			if err != nil {
				return res, err
			}
			n, err = s.deleteBatches(ctx, `
				DELETE FROM vehicle_events WHERE id IN (
					SELECT id FROM vehicle_events WHERE device_id = ANY($1) AND timestamp < $2 LIMIT $3)`, chunk, cutoff)
			res.Events += n
			if err != nil {
				return res, err
			}
		}
	}
	if res.Positions > 0 || res.Events > 0 {
		s.log.Info("histórico vencido apagado", "positions", res.Positions, "events", res.Events)
	}
	return res, nil
}

// deleteBatches repete o DELETE em lotes até não sobrar nada vencido.
func (s *Service) deleteBatches(ctx context.Context, query string, devices []uuid.UUID, cutoff time.Time) (int64, error) {
	var total int64
	for {
		tag, err := s.db.Exec(ctx, query, devices, cutoff, batchSize)
		if err != nil {
			return total, database.MapError(err)
		}
		total += tag.RowsAffected()
		if tag.RowsAffected() < batchSize {
			return total, nil
		}
		select {
		case <-ctx.Done():
			return total, ctx.Err()
		case <-time.After(pause):
		}
	}
}
