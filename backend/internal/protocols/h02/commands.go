package h02

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/protocols"
)

// Comandos do H02 viajam como texto, mesmo quando as posições vêm em binário:
//
//	*HQ,<id>,<tipo>,<hhmmss>[,<param>...]#
//
// NÍVEL DE CONFIANÇA: ASSUMED. Os tipos abaixo vêm da implementação de
// referência, não de documentação do fabricante.
//
// Aviso importante sobre corte de motor: nem todo aparelho H02 tem relé. O
// TK915, por exemplo, é magnético e a bateria — não existe saída de corte
// nele. O comando será aceito e simplesmente não fará nada. Corte de motor
// nesta família é documentado para modelos com relé, como TK920 e TK806.

var safePassword = regexp.MustCompile(`^[A-Za-z0-9]{1,16}$`)

var safeCommandText = regexp.MustCompile(`^[\x20-\x7E]{1,200}$`)

// clock permite congelar o horário nos testes.
var clock = time.Now

func (p *Protocol) EncodeCommand(cmd protocols.Command) ([]byte, error) {
	if raw := strings.TrimSpace(cmd.Raw); raw != "" {
		if !safeCommandText.MatchString(raw) {
			return nil, fmt.Errorf("texto de comando com caractere inválido")
		}
		return []byte(raw), nil
	}

	id := strings.TrimSpace(cmd.UniqueID)
	if err := protocols.ValidateIMEI(id); err != nil {
		return nil, fmt.Errorf("comando H02 exige o identificador do aparelho: %w", err)
	}
	if pwd := strings.TrimSpace(cmd.Password); pwd != "" && !safePassword.MatchString(pwd) {
		return nil, fmt.Errorf("senha do dispositivo com formato inválido")
	}

	var commandType string
	var params []string

	switch cmd.Type {
	case protocols.CommandEngineCut:
		commandType, params = "S20", []string{"1", "1"}
	case protocols.CommandEngineResume:
		commandType, params = "S20", []string{"1", "0"}

	case protocols.CommandSetInterval:
		seconds, err := boundedInt(cmd.Param("seconds"), 5, 18000)
		if err != nil {
			return nil, fmt.Errorf("intervalo: %w", err)
		}
		commandType, params = "S71", []string{"22", strconv.Itoa(seconds)}

	case protocols.CommandCustom:
		return nil, fmt.Errorf("comando custom exige o texto bruto")

	default:
		// Posição, status, reboot e servidor não têm texto confirmado nesta
		// família. Preferimos recusar a chutar bytes (§38).
		return nil, fmt.Errorf("%w: %s não confirmado para o H02",
			protocols.ErrUnsupportedCommand, cmd.Type)
	}

	text := formatCommand(id, commandType, params...)
	if !safeCommandText.MatchString(text) {
		return nil, fmt.Errorf("texto de comando com caractere inválido")
	}
	return []byte(text), nil
}

// formatCommand monta *HQ,<id>,<tipo>,<hhmmss>[,<params>]#
func formatCommand(id, commandType string, params ...string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "*HQ,%s,%s,%s", id, commandType, clock().UTC().Format("150405"))
	for _, param := range params {
		sb.WriteString("," + param)
	}
	sb.WriteString("#")
	return sb.String()
}

func boundedInt(raw string, minValue, maxValue int) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("valor não numérico")
	}
	if value < minValue || value > maxValue {
		return 0, fmt.Errorf("fora da faixa %d..%d", minValue, maxValue)
	}
	return value, nil
}
