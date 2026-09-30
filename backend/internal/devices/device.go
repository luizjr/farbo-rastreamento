// Package devices trata o cadastro dos rastreadores e o seu estado de conexão.
package devices

import (
	"time"

	"github.com/google/uuid"
)

// Status de conexão do dispositivo (§17). Derivado do último pacote recebido,
// não apenas da existência de um socket aberto.
const (
	StatusOnline  = "ONLINE"
	StatusStale   = "STALE"
	StatusOffline = "OFFLINE"
)

type Device struct {
	ID           uuid.UUID `json:"id"`
	IMEI         string    `json:"imei"`
	Model        string    `json:"model"`
	Manufacturer string    `json:"manufacturer"`
	Protocol     string    `json:"protocol"`
	Firmware     string    `json:"firmware"`
	PhoneNumber  string    `json:"phoneNumber"`

	Status     string     `json:"status"`
	LastSeenAt *time.Time `json:"lastSeenAt"`

	// Provisionamento do aparelho (§30).
	APN                      string `json:"apn"`
	APNUser                  string `json:"apnUser"`
	APNPassword              string `json:"apnPassword"`
	ServerHost               string `json:"serverHost"`
	ServerPort               *int   `json:"serverPort"`
	ReportIntervalSeconds    *int   `json:"reportIntervalSeconds"`
	HeartbeatIntervalSeconds *int   `json:"heartbeatIntervalSeconds"`

	// CommandPassword é exigida por alguns firmwares ao receber comandos.
	CommandPassword string `json:"commandPassword"`
	// CommandOverrides substitui o texto padrão de um comando, por tipo.
	// É o escape para firmware divergente sem recompilar nada.
	CommandOverrides map[string]string `json:"commandOverrides"`

	Notes string `json:"notes"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ForCustomer devolve uma cópia sem o que é só da central: credenciais do
// chip e do aparelho, textos de comando, servidor e anotações internas. O
// formato do JSON não muda; os campos só chegam vazios.
func (d *Device) ForCustomer() *Device {
	if d == nil {
		return nil
	}
	return &Device{
		ID:               d.ID,
		IMEI:             d.IMEI,
		Model:            d.Model,
		Manufacturer:     d.Manufacturer,
		Protocol:         d.Protocol,
		Firmware:         d.Firmware,
		Status:           d.Status,
		LastSeenAt:       d.LastSeenAt,
		CommandOverrides: map[string]string{},
		CreatedAt:        d.CreatedAt,
		UpdatedAt:        d.UpdatedAt,
	}
}

// Input é o payload aceito na criação/edição de um dispositivo.
type Input struct {
	IMEI                     string            `json:"imei"`
	Model                    string            `json:"model"`
	Manufacturer             string            `json:"manufacturer"`
	Protocol                 string            `json:"protocol"`
	Firmware                 string            `json:"firmware"`
	PhoneNumber              string            `json:"phoneNumber"`
	APN                      string            `json:"apn"`
	APNUser                  string            `json:"apnUser"`
	APNPassword              string            `json:"apnPassword"`
	ServerHost               string            `json:"serverHost"`
	ServerPort               *int              `json:"serverPort"`
	ReportIntervalSeconds    *int              `json:"reportIntervalSeconds"`
	HeartbeatIntervalSeconds *int              `json:"heartbeatIntervalSeconds"`
	CommandPassword          string            `json:"commandPassword"`
	CommandOverrides         map[string]string `json:"commandOverrides"`
	Notes                    string            `json:"notes"`
}

// StatusChange descreve uma transição detectada pela varredura de status.
type StatusChange struct {
	DeviceID uuid.UUID
	IMEI     string
	From     string
	To       string
}
