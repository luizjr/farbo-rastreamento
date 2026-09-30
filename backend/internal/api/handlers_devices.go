package api

import (
	"errors"
	"maps"
	"net/http"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/audit"
	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/auth"
	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/devices"
)

// deviceAudience escolhe a representação do rastreador para quem chama: o
// admin vê o cadastro (sem as senhas), o resto da equipe só identificação e
// situação, o cliente menos ainda. Nenhum perfil recebe senha (ver
// devices.View).
func deviceAudience(r *http.Request) devices.Audience {
	principal, ok := auth.FromContext(r.Context())
	switch {
	case !ok || principal.IsCustomer():
		return devices.AudienceCustomer
	case principal.CanManage():
		return devices.AudienceAdmin
	}
	return devices.AudienceStaff
}

func (s *Server) handleListDevices(w http.ResponseWriter, r *http.Request) {
	list, err := s.Devices.List(r.Context())
	if err != nil {
		handleStoreError(w, err, "dispositivos não encontrados")
		return
	}
	writeJSON(w, http.StatusOK, devices.Views(list, deviceAudience(r)))
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
	writeJSON(w, http.StatusOK, device.View(deviceAudience(r)))
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
//
// A rota é só do admin. Mesmo para ele os comandos saem com a senha de
// comando trocada por *** (e o device sem senha nenhuma): quem manda o SMS
// digita a senha no lugar — ver devices.ProvisioningCommand.
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
		"device":   device.View(devices.AudienceAdmin),
		"commands": s.Devices.ProvisioningCommands(device),
		"warning": "Confira cada comando no manual do aparelho antes de enviar. " +
			"O sistema não dispara configuração automaticamente. " +
			"Onde aparece " + devices.Redacted + ", digite a senha de comando do aparelho: " +
			"ela não é exibida.",
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
		map[string]any{"imei": device.IMEI, "protocol": device.Protocol,
			"credentials": credentialChanges(nil, device)})
	writeJSON(w, http.StatusCreated, device.View(devices.AudienceAdmin))
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
		map[string]any{"imei": device.IMEI, "credentials": credentialChanges(previous, device)})
	writeJSON(w, http.StatusOK, device.View(devices.AudienceAdmin))
}

// credentialChanges diz à auditoria quais credenciais mudaram — só os nomes,
// nunca os valores. É o rastro de quando cada senha foi trocada.
func credentialChanges(before, after *devices.Device) []string {
	if before == nil {
		before = &devices.Device{}
	}
	changed := []string{}
	if before.APNPassword != after.APNPassword {
		changed = append(changed, "apnPassword")
	}
	if before.CommandPassword != after.CommandPassword {
		changed = append(changed, "commandPassword")
	}
	// Os overrides podem carregar a senha no texto: entram na mesma lista.
	if !maps.Equal(before.CommandOverrides, after.CommandOverrides) {
		changed = append(changed, "commandOverrides")
	}
	return changed
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
