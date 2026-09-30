package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/audit"
	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/retention"
	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/vehicles"
)

type retentionRequest struct {
	// Days é 7, 14 ou 30; nulo volta a herdar (cliente → padrão da central).
	Days *int `json:"days"`
}

func writeRetentionError(w http.ResponseWriter, err error, notFound string) {
	if errors.Is(err, retention.ErrInvalid) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	handleStoreError(w, err, notFound)
}

// handleSetCustomerRetention: a central define por quantos dias guardar o
// histórico de todos os veículos do cliente.
func (s *Server) handleSetCustomerRetention(w http.ResponseWriter, r *http.Request) {
	customerID, ok := s.customerFromURL(w, r)
	if !ok {
		return
	}
	var req retentionRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	if err := s.Retention.SetCustomer(r.Context(), customerID, req.Days); err != nil {
		writeRetentionError(w, err, "cliente não encontrado")
		return
	}
	s.recordBillingAudit(r, audit.ActionRetentionChanged, map[string]any{"customerId": customerID, "days": req.Days})
	writeJSON(w, http.StatusOK, map[string]any{
		"historyRetentionDays": req.Days, "effectiveDays": s.Retention.Effective(nil, req.Days),
	})
}

// handleSetVehicleRetention: exceção de um veículo (nulo segue o cliente).
func (s *Server) handleSetVehicleRetention(w http.ResponseWriter, r *http.Request) {
	vehicle, _, ok := s.vehicleFromURL(w, r, false)
	if !ok {
		return
	}
	var req retentionRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	if err := s.Retention.SetVehicle(r.Context(), vehicle.ID, req.Days); err != nil {
		writeRetentionError(w, err, "veículo não encontrado")
		return
	}
	vehicle.HistoryRetentionDays = req.Days
	days, err := s.historyDays(r.Context(), vehicle)
	if err != nil {
		handleStoreError(w, err, "veículo não encontrado")
		return
	}
	s.recordAudit(r, audit.ActionRetentionChanged, &vehicle.ID, vehicle.DeviceID, map[string]any{"days": req.Days})
	writeJSON(w, http.StatusOK, map[string]any{"historyRetentionDays": req.Days, "effectiveDays": days})
}

// historyDays é o prazo que vale para o veículo.
func (s *Server) historyDays(ctx context.Context, v *vehicles.Vehicle) (int, error) {
	return s.Retention.ForVehicle(ctx, v.HistoryRetentionDays, v.OwnerID)
}

// clampToRetention não deixa a consulta começar antes do que é guardado:
// entre uma limpeza e outra, o que já venceu não aparece.
func (s *Server) clampToRetention(ctx context.Context, v *vehicles.Vehicle, from time.Time) time.Time {
	days, err := s.historyDays(ctx, v)
	if err != nil {
		return from
	}
	if oldest := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour); from.Before(oldest) {
		return oldest
	}
	return from
}
