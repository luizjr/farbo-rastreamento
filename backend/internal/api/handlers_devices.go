package api

import (
	"errors"
	"net/http"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/audit"
	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/devices"
)

func (s *Server) handleListDevices(w http.ResponseWriter, r *http.Request) {
	list, err := s.Devices.List(r.Context())
	if err != nil {
		handleStoreError(w, err, "dispositivos não encontrados")
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleGetDevice(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "id inválido")
		return
	}
	device, err := s.Devices.Get(r.Context(), id)
	if err != nil {
		handleStoreError(w, err, "dispositivo não encontrado")
		return
	}
	writeJSON(w, http.StatusOK, device)
}

// handleDeviceStatus reúne o que se sabe sobre a saúde do rastreador.
func (s *Server) handleDeviceStatus(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "id inválido")
		return
	}
	device, err := s.Devices.Get(r.Context(), id)
	if err != nil {
		handleStoreError(w, err, "dispositivo não encontrado")
		return
	}

	connection, connected := s.Conns.Get(device.IMEI)
	payload := map[string]any{
		"deviceId":   device.ID,
		"status":     device.Status,
		"lastSeenAt": device.LastSeenAt,
		"protocol":   device.Protocol,
		"connected":  connected,
		"state":      s.States.Get(device.ID),
	}
	if connected {
		payload["connection"] = connection.Info()
	}
	if position, err := s.Positions.Latest(r.Context(), device.ID); err == nil {
		payload["lastPosition"] = position
	}
	writeJSON(w, http.StatusOK, payload)
}

// handleDeviceProvisioning mostra os comandos de configuração sugeridos.
// Nada é enviado automaticamente: o operador confirma (§30).
func (s *Server) handleDeviceProvisioning(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "id inválido")
		return
	}
	device, err := s.Devices.Get(r.Context(), id)
	if err != nil {
		handleStoreError(w, err, "dispositivo não encontrado")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"device":   device,
		"commands": s.Devices.ProvisioningCommands(device),
		"warning": "Confira cada comando no manual do aparelho antes de enviar. " +
			"O sistema não dispara configuração automaticamente.",
	})
}

func (s *Server) handleCreateDevice(w http.ResponseWriter, r *http.Request) {
	var in devices.Input
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}

	device, err := s.Devices.Create(r.Context(), in)
	if err != nil {
		var validation devices.ValidationError
		if errors.As(err, &validation) {
			writeError(w, http.StatusBadRequest, validation.Message)
			return
		}
		handleStoreError(w, err, "dispositivo não encontrado")
		return
	}

	s.Ingestor.InvalidateDevice(device.IMEI)
	s.recordAudit(r, audit.ActionDeviceCreated, nil, &device.ID,
		map[string]any{"imei": device.IMEI, "protocol": device.Protocol})
	writeJSON(w, http.StatusCreated, device)
}

func (s *Server) handleUpdateDevice(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "id inválido")
		return
	}

	previous, err := s.Devices.Get(r.Context(), id)
	if err != nil {
		handleStoreError(w, err, "dispositivo não encontrado")
		return
	}

	var in devices.Input
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}

	device, err := s.Devices.Update(r.Context(), id, in)
	if err != nil {
		var validation devices.ValidationError
		if errors.As(err, &validation) {
			writeError(w, http.StatusBadRequest, validation.Message)
			return
		}
		handleStoreError(w, err, "dispositivo não encontrado")
		return
	}

	s.Ingestor.InvalidateDevice(previous.IMEI)
	s.Ingestor.InvalidateDevice(device.IMEI)
	s.recordAudit(r, audit.ActionDeviceUpdated, nil, &device.ID,
		map[string]any{"imei": device.IMEI})
	writeJSON(w, http.StatusOK, device)
}

func (s *Server) handleDeleteDevice(w http.ResponseWriter, r *http.Request) {
	id, err := urlUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "id inválido")
		return
	}

	device, err := s.Devices.Get(r.Context(), id)
	if err != nil {
		handleStoreError(w, err, "dispositivo não encontrado")
		return
	}
	if err := s.Devices.Delete(r.Context(), id); err != nil {
		handleStoreError(w, err, "dispositivo não encontrado")
		return
	}

	s.Ingestor.InvalidateDevice(device.IMEI)
	s.Ingestor.InvalidateVehicles()
	s.recordAudit(r, audit.ActionDeviceDeleted, nil, &id,
		map[string]any{"imei": device.IMEI})
	writeJSON(w, http.StatusNoContent, nil)
}
