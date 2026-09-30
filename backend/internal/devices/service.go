package devices

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/protocols"
)

type Service struct {
	repo     *Repository
	registry *protocols.ProtocolRegistry
}

func NewService(repo *Repository, registry *protocols.ProtocolRegistry) *Service {
	return &Service{repo: repo, registry: registry}
}

func (s *Service) List(ctx context.Context) ([]*Device, error) { return s.repo.List(ctx) }

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Device, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) GetByIMEI(ctx context.Context, imei string) (*Device, error) {
	return s.repo.GetByIMEI(ctx, imei)
}

func (s *Service) Create(ctx context.Context, in Input) (*Device, error) {
	normalized, err := s.validate(in)
	if err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, normalized)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, in Input) (*Device, error) {
	normalized, err := s.validate(in)
	if err != nil {
		return nil, err
	}
	return s.repo.Update(ctx, id, normalized)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *Service) Touch(ctx context.Context, id uuid.UUID, seenAt time.Time) error {
	return s.repo.Touch(ctx, id, seenAt)
}

func (s *Service) SetProtocol(ctx context.Context, id uuid.UUID, protocol string) error {
	return s.repo.SetProtocol(ctx, id, protocol)
}

func (s *Service) SweepStatuses(ctx context.Context, stale, offline time.Duration) ([]StatusChange, error) {
	return s.repo.SweepStatuses(ctx, stale, offline)
}

func (s *Service) CountOnline(ctx context.Context) (int, error) { return s.repo.CountOnline(ctx) }

// ValidationError descreve um payload recusado.
type ValidationError struct{ Message string }

func (e ValidationError) Error() string { return e.Message }

func invalid(format string, args ...any) error {
	return ValidationError{Message: fmt.Sprintf(format, args...)}
}

func (s *Service) validate(in Input) (Input, error) {
	in.IMEI = strings.TrimSpace(in.IMEI)
	if err := protocols.ValidateIMEI(in.IMEI); err != nil {
		return in, invalid("IMEI inválido: %v", err)
	}

	in.Protocol = strings.ToLower(strings.TrimSpace(in.Protocol))
	if in.Protocol != "" {
		if _, ok := s.registry.ByName(in.Protocol); !ok {
			return in, invalid("protocolo %q não está registrado", in.Protocol)
		}
	}

	if in.ServerPort != nil && (*in.ServerPort < 1 || *in.ServerPort > 65535) {
		return in, invalid("porta do servidor fora da faixa 1..65535")
	}
	if in.ReportIntervalSeconds != nil && (*in.ReportIntervalSeconds < 5 || *in.ReportIntervalSeconds > 86400) {
		return in, invalid("intervalo de envio fora da faixa 5..86400 segundos")
	}
	if in.HeartbeatIntervalSeconds != nil && (*in.HeartbeatIntervalSeconds < 30 || *in.HeartbeatIntervalSeconds > 86400) {
		return in, invalid("intervalo de heartbeat fora da faixa 30..86400 segundos")
	}

	// Os overrides são texto bruto enviado ao aparelho: sanitizamos aqui para
	// que não seja possível injetar comandos extras pelo cadastro (§27).
	clean := map[string]string{}
	for key, value := range in.CommandOverrides {
		cmdType := protocols.CommandType(strings.ToUpper(strings.TrimSpace(key)))
		if !cmdType.Valid() {
			return in, invalid("override para comando desconhecido: %q", key)
		}
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if len(value) > 200 {
			return in, invalid("override de %s é longo demais", cmdType)
		}
		for _, r := range value {
			if r < 0x20 || r > 0x7E {
				return in, invalid("override de %s contém caractere de controle", cmdType)
			}
		}
		clean[string(cmdType)] = value
	}
	in.CommandOverrides = clean

	if pwd := strings.TrimSpace(in.CommandPassword); pwd != "" {
		for _, r := range pwd {
			isDigit := r >= '0' && r <= '9'
			isLetter := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
			if !isDigit && !isLetter {
				return in, invalid("senha de comando aceita apenas letras e números")
			}
		}
		if len(pwd) > 16 {
			return in, invalid("senha de comando longa demais")
		}
		in.CommandPassword = pwd
	}

	return in, nil
}

// ProvisioningCommands devolve os comandos sugeridos para apontar o aparelho
// para este servidor. Nada é enviado automaticamente (§30): a interface mostra
// o texto e o operador decide.
type ProvisioningCommand struct {
	Type        protocols.CommandType `json:"type"`
	Description string                `json:"description"`
	Text        string                `json:"text"`
	Available   bool                  `json:"available"`
	Reason      string                `json:"reason,omitempty"`
}

func (s *Service) ProvisioningCommands(dev *Device) []ProvisioningCommand {
	out := []ProvisioningCommand{}
	proto, ok := s.registry.ByName(dev.Protocol)
	if !ok {
		return out
	}

	build := func(cmdType protocols.CommandType, description string, params map[string]string) {
		entry := ProvisioningCommand{Type: cmdType, Description: description}
		raw, err := proto.EncodeCommand(protocols.Command{
			Type:     cmdType,
			UniqueID: dev.IMEI,
			Password: dev.CommandPassword,
			Params:   params,
			Raw:      dev.CommandOverrides[string(cmdType)],
		})
		if err != nil {
			entry.Reason = err.Error()
		} else {
			entry.Available = true
			entry.Text = printableCommand(raw)
		}
		out = append(out, entry)
	}

	if dev.ServerHost != "" && dev.ServerPort != nil {
		build(protocols.CommandSetServer, "Aponta o rastreador para este servidor",
			map[string]string{"host": dev.ServerHost, "port": fmt.Sprint(*dev.ServerPort)})
	}
	if dev.ReportIntervalSeconds != nil {
		build(protocols.CommandSetInterval, "Define o intervalo de envio de posição",
			map[string]string{"seconds": fmt.Sprint(*dev.ReportIntervalSeconds)})
	}
	if dev.HeartbeatIntervalSeconds != nil {
		build(protocols.CommandSetHeartbeat, "Define o intervalo de heartbeat",
			map[string]string{"minutes": fmt.Sprint(*dev.HeartbeatIntervalSeconds / 60)})
	}
	return out
}

// printableCommand mostra o comando de forma legível, sem exibir bytes crus.
func printableCommand(raw []byte) string {
	var sb strings.Builder
	for _, b := range raw {
		if b >= 0x20 && b <= 0x7E {
			sb.WriteByte(b)
		} else {
			fmt.Fprintf(&sb, "\\x%02X", b)
		}
	}
	return sb.String()
}
