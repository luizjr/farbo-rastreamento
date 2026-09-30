package api

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/audit"
)

// handleConnections lista as sessões TCP abertas com rastreadores.
func (s *Server) handleConnections(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"count":       s.Conns.Count(),
		"connections": s.Conns.List(),
	})
}

// handleRawPackets expõe o tráfego que nenhum protocolo soube interpretar.
//
// É esta tela que transforma "não sei qual variante o TK970 fala" em
// "aqui estão os bytes que ele mandou" (§38).
func (s *Server) handleRawPackets(w http.ResponseWriter, r *http.Request) {
	list, err := s.Raw.List(r.Context(), queryInt(r, "limit", 100))
	if err != nil {
		handleStoreError(w, err, "captura não encontrada")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"packets": list,
		"hint": "Payload legível começando com '*' ou 'imei:' indica protocolo de texto; " +
			"começando com 7878/7979 indica GT06. Use isto para confirmar a variante " +
			"antes de implementar o parser.",
	})
}

func (s *Server) handleAuditLogs(w http.ResponseWriter, r *http.Request) {
	query := audit.Query{Limit: queryInt(r, "limit", 100)}

	if raw := r.URL.Query().Get("deviceId"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "deviceId inválido")
			return
		}
		query.DeviceID = &id
	}
	if raw := r.URL.Query().Get("vehicleId"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "vehicleId inválido")
			return
		}
		query.VehicleID = &id
	}

	list, err := s.Audit.List(r.Context(), query)
	if err != nil {
		handleStoreError(w, err, "auditoria não encontrada")
		return
	}
	writeJSON(w, http.StatusOK, list)
}
