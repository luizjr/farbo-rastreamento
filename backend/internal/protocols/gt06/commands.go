package gt06

import (
	"encoding/binary"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/protocols"
)

// serialCounter numera os quadros que o servidor envia. O GT06 não exige
// ordem no número de série do servidor, apenas que ele exista.
var serialCounter atomic.Uint32

func nextSerial() uint16 {
	return uint16(serialCounter.Add(1))
}

// maxCommandText limita o texto ASCII cabível em um pacote 0x80.
const maxCommandText = 200

// safeHost aceita apenas host/IP plausível, bloqueando injeção no texto do
// comando (§27).
var safeHost = regexp.MustCompile(`^[A-Za-z0-9.\-]{1,64}$`)

// safeCustom recusa caracteres de controle em comandos livres.
var safeCustom = regexp.MustCompile(`^[\x20-\x7E]{1,200}$`)

// EncodeCommand traduz o comando canônico para um pacote 0x80.
//
// Os textos abaixo seguem o conjunto de comandos online do GT06/Concox.
// Quando o dispositivo tiver override cadastrado, ele chega aqui em cmd.Raw e
// tem precedência — é assim que se acomoda firmware divergente sem recompilar.
func (p *Protocol) EncodeCommand(cmd protocols.Command) ([]byte, error) {
	text, err := commandText(cmd)
	if err != nil {
		return nil, err
	}
	if !safeCustom.MatchString(text) {
		return nil, fmt.Errorf("texto de comando com caractere inválido")
	}
	if len(text) > maxCommandText {
		return nil, fmt.Errorf("comando com %d caracteres excede o limite de %d", len(text), maxCommandText)
	}

	// DOCUMENTED: conteúdo do 0x80 = tamanho (1) + server flag (4) +
	// comando ASCII + idioma (2).
	content := make([]byte, 0, 7+len(text))
	content = append(content, byte(len(text)+4))
	content = binary.BigEndian.AppendUint32(content, cmd.CorrelationKey)
	content = append(content, text...)
	content = append(content, 0x00, 0x02) // idioma: inglês

	return buildFrame(msgCommand, content, nextSerial()), nil
}

func commandText(cmd protocols.Command) (string, error) {
	if raw := strings.TrimSpace(cmd.Raw); raw != "" {
		return raw, nil
	}

	pwd := strings.TrimSpace(cmd.Password)
	if pwd != "" && !regexp.MustCompile(`^[A-Za-z0-9]{1,16}$`).MatchString(pwd) {
		return "", fmt.Errorf("senha do dispositivo com formato inválido")
	}

	// withPwd monta "CMD#" ou "CMD,senha#" conforme o dispositivo exija senha.
	withPwd := func(base string) string {
		if pwd == "" {
			return base + "#"
		}
		return base + "," + pwd + "#"
	}

	switch cmd.Type {
	case protocols.CommandEngineCut:
		// DOCUMENTED: corta óleo/energia.
		return withPwd("DYD"), nil
	case protocols.CommandEngineResume:
		// DOCUMENTED: restaura óleo/energia.
		return withPwd("HFYD"), nil
	case protocols.CommandRequestPosition:
		// DOCUMENTED.
		return withPwd("WHERE"), nil
	case protocols.CommandRequestStatus:
		// DOCUMENTED.
		return withPwd("STATUS"), nil
	case protocols.CommandReboot:
		// DOCUMENTED.
		return withPwd("RESET"), nil

	case protocols.CommandSetInterval:
		// TODO: VERIFY AGAINST DEVICE PROTOCOL — "TIMER" existe em boa parte
		// da linha Concox, mas há firmwares que usam "UPLOAD". Confirme no
		// manual do seu aparelho e, se divergir, cadastre um override.
		seconds, err := positiveInt(cmd.Param("seconds"), 5, 18000)
		if err != nil {
			return "", fmt.Errorf("intervalo: %w", err)
		}
		if pwd == "" {
			return fmt.Sprintf("TIMER,%d#", seconds), nil
		}
		return fmt.Sprintf("TIMER,%s,%d#", pwd, seconds), nil

	case protocols.CommandSetHeartbeat:
		// TODO: VERIFY AGAINST DEVICE PROTOCOL — "HBT" em minutos aparece na
		// documentação Concox de alguns modelos; não é universal.
		minutes, err := positiveInt(cmd.Param("minutes"), 1, 360)
		if err != nil {
			return "", fmt.Errorf("heartbeat: %w", err)
		}
		return fmt.Sprintf("HBT,%d#", minutes), nil

	case protocols.CommandSetServer:
		// DOCUMENTED: SERVER,<modo>,<host>,<porta>,<0>#  (modo 1 = domínio/IP)
		host := strings.TrimSpace(cmd.Param("host"))
		if !safeHost.MatchString(host) {
			return "", fmt.Errorf("host inválido")
		}
		port, err := positiveInt(cmd.Param("port"), 1, 65535)
		if err != nil {
			return "", fmt.Errorf("porta: %w", err)
		}
		return fmt.Sprintf("SERVER,1,%s,%d,0#", host, port), nil

	case protocols.CommandCustom:
		return "", fmt.Errorf("comando custom exige o texto bruto")
	default:
		return "", protocols.ErrUnsupportedCommand
	}
}

func positiveInt(raw string, min, max int) (int, error) {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("valor não numérico")
	}
	if v < min || v > max {
		return 0, fmt.Errorf("fora da faixa %d..%d", min, max)
	}
	return v, nil
}

// buildTimeReply responde ao pedido de sincronismo (0x8A) com a hora UTC.
func buildTimeReply(serial uint16) []byte {
	now := time.Now().UTC()
	content := []byte{
		byte(now.Year() - 2000), byte(now.Month()), byte(now.Day()),
		byte(now.Hour()), byte(now.Minute()), byte(now.Second()),
	}
	return buildFrame(msgTimeRequest, content, serial)
}
