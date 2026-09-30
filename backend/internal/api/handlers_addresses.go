package api

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/addresses"
	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/audit"
)

// handleSaveMyAddress: o cliente cadastra ou troca o endereço de entrega.
// Fica liberado mesmo com o acesso suspenso — é dado de cadastro.
func (s *Server) handleSaveMyAddress(w http.ResponseWriter, r *http.Request) {
	customerID, _ := customerOf(r)
	s.saveAddress(w, r, customerID, "CLIENTE")
}

// handleSaveCustomerAddress: a central ajusta o endereço de entrega do cliente.
func (s *Server) handleSaveCustomerAddress(w http.ResponseWriter, r *http.Request) {
	customerID, ok := s.customerFromURL(w, r)
	if !ok {
		return
	}
	s.saveAddress(w, r, customerID, "CENTRAL")
}

func (s *Server) saveAddress(w http.ResponseWriter, r *http.Request, customerID uuid.UUID, origin string) {
	var in addresses.Address
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	in, err := addresses.Normalize(in)
	var validation addresses.ValidationError
	if errors.As(err, &validation) {
		writeError(w, http.StatusBadRequest, validation.Message)
		return
	}

	saved, err := s.Addresses.Save(r.Context(), customerID, in)
	if err != nil {
		handleStoreError(w, err, "cliente não encontrado")
		return
	}
	// O endereço é dado pessoal: a auditoria registra só quem mudou e a cidade.
	s.recordAudit(r, audit.ActionDeliveryAddressSaved, nil, nil, map[string]any{
		"customerId": customerID, "origin": origin, "city": saved.City, "state": saved.State,
	})
	writeJSON(w, http.StatusOK, saved)
}
