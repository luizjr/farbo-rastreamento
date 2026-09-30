package api

import (
	"errors"
	"net/http"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/audit"
	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/installers"
)

// handlePublicInstallers lista os prestadores ativos para a landing page e o
// painel do cliente. Rota pública: sem login e sem dados de controle.
func (s *Server) handlePublicInstallers(w http.ResponseWriter, r *http.Request) {
	list, err := s.Installers.ListActive(r.Context())
	if err != nil {
		handleStoreError(w, err, "prestadores não encontrados")
		return
	}
	out := make([]installers.Public, 0, len(list))
	for _, i := range list {
		out = append(out, i.Public())
	}
	// A lista muda pouco: um minuto de cache poupa o banco em dia de tráfego.
	w.Header().Set("Cache-Control", "public, max-age=60")
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleListInstallers(w http.ResponseWriter, r *http.Request) {
	list, err := s.Installers.List(r.Context())
	if err != nil {
		handleStoreError(w, err, "prestadores não encontrados")
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateInstaller(w http.ResponseWriter, r *http.Request) {
	in, ok := s.installerInput(w, r)
	if !ok {
		return
	}
	created, err := s.Installers.Create(r.Context(), in)
	if err != nil {
		handleStoreError(w, err, "prestador não encontrado")
		return
	}
	s.recordAudit(r, audit.ActionInstallerChanged, nil, nil,
		map[string]any{"installerId": created.ID, "name": created.Name, "op": "create"})
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleUpdateInstaller(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "id inválido")
		return
	}
	in, ok := s.installerInput(w, r)
	if !ok {
		return
	}
	updated, err := s.Installers.Update(r.Context(), id, in)
	if err != nil {
		handleStoreError(w, err, "prestador não encontrado")
		return
	}
	s.recordAudit(r, audit.ActionInstallerChanged, nil, nil,
		map[string]any{"installerId": updated.ID, "name": updated.Name, "op": "update", "active": updated.Active})
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteInstaller(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "id inválido")
		return
	}
	if err := s.Installers.Delete(r.Context(), id); err != nil {
		handleStoreError(w, err, "prestador não encontrado")
		return
	}
	s.recordAudit(r, audit.ActionInstallerChanged, nil, nil,
		map[string]any{"installerId": id, "op": "delete"})
	writeJSON(w, http.StatusNoContent, nil)
}

// installerInput lê e valida o cadastro do corpo da requisição.
func (s *Server) installerInput(w http.ResponseWriter, r *http.Request) (installers.Input, bool) {
	var in installers.Input
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return in, false
	}
	normalized, err := installers.Normalize(in)
	if err != nil {
		var validation installers.ValidationError
		if errors.As(err, &validation) {
			writeError(w, http.StatusBadRequest, validation.Message)
			return in, false
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return in, false
	}
	return normalized, true
}
