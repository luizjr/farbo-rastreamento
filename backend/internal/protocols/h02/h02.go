// Package h02 implementa o protocolo H02, usado pela linha TKSTAR (TK905,
// TK915, TK917, TK920) e por vários outros aparelhos chineses.
//
// NÍVEL DE CONFIANÇA: ASSUMED
//
// O layout dos campos aqui foi obtido de duas fontes independentes que
// concordam entre si — a implementação de referência aberta do protocolo e uma
// captura real de um TK905 4G — mas NÃO de documentação do fabricante. A
// captura decodifica para uma coordenada real e coerente (Melbourne, com
// velocidade zero, rumo 344° e ignição desligada), o que valida a ordem e o
// significado dos campos. Ainda assim, dois pontos merecem conferência com o
// aparelho em mãos:
//
//  1. A unidade do campo de velocidade (tratada aqui como nós).
//  2. O tamanho do quadro binário — ver o comentário de NextFrame.
//
// O protocolo tem duas formas que convivem na mesma conexão:
//
//	'$' → quadro binário, sem terminador e sem campo de tamanho (posições)
//	'*' → quadro de texto terminado em '#' (heartbeat e respostas de comando)
package h02

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/protocols"
)

const Name = "h02"

const (
	markerBinary = '$'
	markerText   = '*'
)

// maxFrame protege contra quadros absurdos antes de alocar.
const maxFrame = 1024

// Protocol implementa protocols.TrackerProtocol.
type Protocol struct {
	// BinaryFrameLength fixa o tamanho do quadro binário.
	//
	// Zero liga a heurística descrita em NextFrame. Depois de observar o
	// tamanho real do seu aparelho (ele aparece em Attributes["frameBytes"] e
	// na captura crua), vale fixar aqui via H02_BINARY_FRAME_LENGTH.
	BinaryFrameLength int
}

func New(binaryFrameLength int) *Protocol {
	return &Protocol{BinaryFrameLength: binaryFrameLength}
}

func (p *Protocol) Name() string { return Name }

func (p *Protocol) Describe() protocols.Descriptor {
	return protocols.Descriptor{
		Name:       Name,
		Label:      "H02 — binário e texto (TKSTAR TK905/TK915/TK920)",
		Vendor:     "TKSTAR e compatíveis",
		Confidence: protocols.Assumed,
		Commands: []protocols.CommandType{
			protocols.CommandEngineCut, protocols.CommandEngineResume,
			protocols.CommandSetInterval, protocols.CommandCustom,
		},
		Notes: "Layout confirmado contra implementação de referência e uma captura real, " +
			"não contra documentação do fabricante. Confira a unidade de velocidade e o " +
			"tamanho do quadro binário com o aparelho antes de operar o corte de motor. " +
			"O protocolo não carrega chave de correlação: o ACK casa com o comando aberto " +
			"mais antigo. Atenção ao cadastro: estes aparelhos costumam reportar um " +
			"identificador curto, não o IMEI de 15 dígitos.",
	}
}

func (p *Protocol) Detect(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	switch data[0] {
	case markerBinary:
		// Um quadro binário precisa ter ao menos o bloco decodificável; com
		// menos bytes ainda não dá para afirmar nada, mas o marcador basta
		// para reivindicar a conexão.
		return true
	case markerText:
		// Exige um identificador plausível no segundo campo, para não roubar
		// tráfego de outros protocolos que também começam com '*'.
		parts := strings.Split(strings.TrimSuffix(string(data), "#"), ",")
		if len(parts) < 2 {
			return len(data) < 32 // pode estar chegando pela metade
		}
		return protocols.ValidateIMEI(strings.TrimSpace(parts[1])) == nil
	default:
		return false
	}
}

// NextFrame separa um quadro do fluxo.
//
// O quadro de texto é simples: termina em '#'.
//
// O binário é o problema do protocolo: ele não tem terminador nem campo de
// tamanho, então só é possível saber onde termina sabendo o tamanho de
// antemão. Com BinaryFrameLength fixado, usamos esse valor. Sem ele, a
// heurística é assumir que o aparelho manda um quadro por segmento TCP — o que
// é o comportamento observado — e entregar o buffer inteiro.
//
// A rede de proteção contra enquadramento errado não está aqui, e sim no
// decodificador: todo nibble BCD é validado e data, coordenada e rumo passam
// por faixa. Quadro cortado no lugar errado vira erro e captura crua, não uma
// posição plausível e falsa.
func (p *Protocol) NextFrame(buf []byte) ([]byte, int, error) {
	start := indexOfMarker(buf)
	switch {
	case start < 0:
		if len(buf) > maxFrame {
			return nil, len(buf), fmt.Errorf("%d bytes sem marcador H02", len(buf))
		}
		if len(buf) == 0 {
			return nil, 0, protocols.ErrIncompleteFrame
		}
		return nil, len(buf), fmt.Errorf("%d bytes sem marcador H02", len(buf))
	case start > 0:
		return nil, start, fmt.Errorf("%d bytes descartados antes do marcador", start)
	}

	if buf[0] == markerText {
		end := indexOfByte(buf, '#')
		if end < 0 {
			if len(buf) > maxFrame {
				return nil, len(buf), fmt.Errorf("quadro de texto sem terminador")
			}
			return nil, 0, protocols.ErrIncompleteFrame
		}
		return buf[:end+1], end + 1, nil
	}

	// Quadro binário.
	if p.BinaryFrameLength > 0 {
		if p.BinaryFrameLength > maxFrame {
			return nil, len(buf), fmt.Errorf("tamanho de quadro configurado é absurdo")
		}
		if len(buf) < p.BinaryFrameLength {
			return nil, 0, protocols.ErrIncompleteFrame
		}
		return buf[:p.BinaryFrameLength], p.BinaryFrameLength, nil
	}

	if len(buf) < minBinaryPayload {
		return nil, 0, protocols.ErrIncompleteFrame
	}
	if len(buf) > maxFrame {
		return nil, len(buf), fmt.Errorf("quadro binário acima do limite")
	}
	return buf, len(buf), nil
}

func (p *Protocol) Parse(frame []byte) ([]protocols.TrackerMessage, error) {
	if len(frame) == 0 {
		return nil, fmt.Errorf("quadro vazio")
	}

	var msg protocols.TrackerMessage
	var err error

	switch frame[0] {
	case markerBinary:
		msg, err = decodeBinary(frame)
	case markerText:
		msg, err = decodeText(frame)
	default:
		return nil, fmt.Errorf("marcador H02 desconhecido: %02X", frame[0])
	}
	if err != nil {
		return nil, err
	}

	return []protocols.TrackerMessage{msg}, nil
}

func indexOfMarker(buf []byte) int {
	for i, b := range buf {
		if b == markerBinary || b == markerText {
			return i
		}
	}
	return -1
}

func indexOfByte(buf []byte, target byte) int {
	for i, b := range buf {
		if b == target {
			return i
		}
	}
	return -1
}

func hexdump(b []byte) string { return strings.ToUpper(hex.EncodeToString(b)) }
