package tkstar

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/protocols"
)

// TKSTARProtocol expõe os comandos do aparelho como funções, em vez de espalhar
// strings pelo código (§15). Cada variante pode devolver textos diferentes.
type TKSTARProtocol interface {
	protocols.TrackerProtocol

	EngineCut() []byte
	EngineResume() []byte
	RequestPosition() []byte
	RequestStatus() []byte
}

// commandSet é o vocabulário de comandos de uma variante do firmware.
//
// Os quatro primeiros seguem o conjunto informado na especificação do projeto
// para os modelos com relé (TK910/TK970). Os demais estão marcados abaixo.
type commandSet struct {
	engineCut       string
	engineResume    string
	requestPosition string
	requestStatus   string
	reboot          string

	// setInterval e setHeartbeat recebem o valor já validado.
	setInterval  func(seconds int) string
	setHeartbeat func(minutes int) string
	setServer    func(host string, port int) string

	// terminator é acrescentado quando o texto ainda não o traz.
	terminator string
}

// defaultCommands é o conjunto citado na documentação do projeto para os
// TKSTAR com relé. Firmware divergente se resolve com override no cadastro do
// dispositivo, sem recompilar.
func defaultCommands() commandSet {
	return commandSet{
		engineCut:       "RELAY,1#",
		engineResume:    "RELAY,0#",
		requestPosition: "WHERE#",
		requestStatus:   "STATUS#",
		// TODO: VERIFY AGAINST DEVICE PROTOCOL — comandos abaixo não foram
		// confirmados para TK910/TK970; são recusados até que se confirme.
		reboot:       "",
		setInterval:  nil,
		setHeartbeat: nil,
		setServer:    nil,
		terminator:   "#",
	}
}

// safeCommandText recusa caracteres de controle, protegendo contra injeção no
// fluxo de texto enviado ao rastreador (§27).
var safeCommandText = regexp.MustCompile(`^[\x20-\x7E]{1,200}$`)

var safeHost = regexp.MustCompile(`^[A-Za-z0-9.\-]{1,64}$`)

var safePassword = regexp.MustCompile(`^[A-Za-z0-9]{1,16}$`)

// encodeCommand traduz o comando canônico usando o vocabulário da variante.
func encodeCommand(set commandSet, cmd protocols.Command) ([]byte, error) {
	text, err := commandText(set, cmd)
	if err != nil {
		return nil, err
	}
	if set.terminator != "" && !strings.HasSuffix(text, set.terminator) {
		text += set.terminator
	}
	if !safeCommandText.MatchString(text) {
		return nil, fmt.Errorf("texto de comando com caractere inválido")
	}
	return []byte(text), nil
}

func commandText(set commandSet, cmd protocols.Command) (string, error) {
	// O override cadastrado no dispositivo tem precedência sobre o padrão.
	if raw := strings.TrimSpace(cmd.Raw); raw != "" {
		return raw, nil
	}
	if pwd := strings.TrimSpace(cmd.Password); pwd != "" && !safePassword.MatchString(pwd) {
		return "", fmt.Errorf("senha do dispositivo com formato inválido")
	}

	switch cmd.Type {
	case protocols.CommandEngineCut:
		return required(set.engineCut, cmd.Type)
	case protocols.CommandEngineResume:
		return required(set.engineResume, cmd.Type)
	case protocols.CommandRequestPosition:
		return required(set.requestPosition, cmd.Type)
	case protocols.CommandRequestStatus:
		return required(set.requestStatus, cmd.Type)
	case protocols.CommandReboot:
		return required(set.reboot, cmd.Type)

	case protocols.CommandSetInterval:
		if set.setInterval == nil {
			return "", unconfirmed(cmd.Type)
		}
		seconds, err := boundedInt(cmd.Param("seconds"), 5, 18000)
		if err != nil {
			return "", fmt.Errorf("intervalo: %w", err)
		}
		return set.setInterval(seconds), nil

	case protocols.CommandSetHeartbeat:
		if set.setHeartbeat == nil {
			return "", unconfirmed(cmd.Type)
		}
		minutes, err := boundedInt(cmd.Param("minutes"), 1, 360)
		if err != nil {
			return "", fmt.Errorf("heartbeat: %w", err)
		}
		return set.setHeartbeat(minutes), nil

	case protocols.CommandSetServer:
		if set.setServer == nil {
			return "", unconfirmed(cmd.Type)
		}
		host := strings.TrimSpace(cmd.Param("host"))
		if !safeHost.MatchString(host) {
			return "", fmt.Errorf("host inválido")
		}
		port, err := boundedInt(cmd.Param("port"), 1, 65535)
		if err != nil {
			return "", fmt.Errorf("porta: %w", err)
		}
		return set.setServer(host, port), nil

	case protocols.CommandCustom:
		return "", fmt.Errorf("comando custom exige o texto bruto")
	default:
		return "", protocols.ErrUnsupportedCommand
	}
}

func required(text string, cmdType protocols.CommandType) (string, error) {
	if strings.TrimSpace(text) == "" {
		return "", unconfirmed(cmdType)
	}
	return text, nil
}

// unconfirmed é o erro devolvido quando o comando existe no domínio mas não há
// texto confirmado para esta variante. Preferimos falhar a chutar bytes.
func unconfirmed(cmdType protocols.CommandType) error {
	return fmt.Errorf("%w: %s não confirmado para esta variante TKSTAR; "+
		"cadastre um override no dispositivo após conferir o manual",
		protocols.ErrUnsupportedCommand, cmdType)
}

func boundedInt(raw string, minValue, maxValue int) (int, error) {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("valor não numérico")
	}
	if v < minValue || v > maxValue {
		return 0, fmt.Errorf("fora da faixa %d..%d", minValue, maxValue)
	}
	return v, nil
}

// supportedCommands lista os comandos com texto confirmado na variante.
func supportedCommands(set commandSet) []protocols.CommandType {
	out := []protocols.CommandType{}
	if set.engineCut != "" {
		out = append(out, protocols.CommandEngineCut)
	}
	if set.engineResume != "" {
		out = append(out, protocols.CommandEngineResume)
	}
	if set.requestPosition != "" {
		out = append(out, protocols.CommandRequestPosition)
	}
	if set.requestStatus != "" {
		out = append(out, protocols.CommandRequestStatus)
	}
	if set.reboot != "" {
		out = append(out, protocols.CommandReboot)
	}
	if set.setInterval != nil {
		out = append(out, protocols.CommandSetInterval)
	}
	if set.setHeartbeat != nil {
		out = append(out, protocols.CommandSetHeartbeat)
	}
	if set.setServer != nil {
		out = append(out, protocols.CommandSetServer)
	}
	return append(out, protocols.CommandCustom)
}
