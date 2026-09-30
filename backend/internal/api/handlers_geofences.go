package api

import (
	"errors"
	"net/http"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/audit"
	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/geofences"
)

func (s *Server) handleListGeofences(w http.ResponseWriter, r *http.Request) {
	// As cercas são da operação da central; o cliente não as vê.
	if _, isCustomer := customerOf(r); isCustomer {
		writeJSON(w, http.StatusOK, []struct{}{})
		return
	}
	list, err := s.Geofences.List(r.Context())
	if err != nil {
		handleStoreError(w, err, "cercas não encontradas")
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateGeofence(w http.ResponseWriter, r *http.Request) {
	var in geofences.Input
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}

	fence, err := s.Geofences.Create(r.Context(), in)
	if err != nil {
		var validation geofences.ValidationError
		if errors.As(err, &validation) {
			writeError(w, http.StatusBadRequest, validation.Message)
			return
		}
		handleStoreError(w, err, "cerca não encontrada")
		return
	}

	s.recordAudit(r, audit.ActionGeofenceChanged, nil, nil,
		map[string]any{"action": "create", "geofenceId": fence.ID, "name": fence.Name})
	writeJSON(w, http.StatusCreated, fence)
}

func (s *Server) handleUpdateGeofence(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "id inválido")
		return
	}

	var in geofences.Input
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}

	fence, err := s.Geofences.Update(r.Context(), id, in)
	if err != nil {
		var validation geofences.ValidationError
		if errors.As(err, &validation) {
			writeError(w, http.StatusBadRequest, validation.Message)
			return
		}
		handleStoreError(w, err, "cerca não encontrada")
		return
	}

	s.recordAudit(r, audit.ActionGeofenceChanged, nil, nil,
		map[string]any{"action": "update", "geofenceId": fence.ID})
	writeJSON(w, http.StatusOK, fence)
}

func (s *Server) handleDeleteGeofence(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "id inválido")
		return
	}
	if err := s.Geofences.Delete(r.Context(), id); err != nil {
		handleStoreError(w, err, "cerca não encontrada")
		return
	}

	s.recordAudit(r, audit.ActionGeofenceChanged, nil, nil,
		map[string]any{"action": "delete", "geofenceId": id})
	writeJSON(w, http.StatusNoContent, nil)
}
