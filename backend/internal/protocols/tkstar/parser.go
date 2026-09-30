// Package tkstar implementa os dialetos de texto usados pela linha TKSTAR.
//
// # LEIA ISTO ANTES DE CONFIAR EM QUALQUER COISA AQUI
//
// Não existe uma especificação pública única para "o protocolo TKSTAR": o nome
// cobre aparelhos de fabricantes diferentes, com firmwares diferentes. Por isso
// cada variante deste pacote declara explicitamente o seu nível de confiança:
//
//	protocol_v1.go  ASSUMED  gramática de texto "imei:...;" (família GPS103/TK103)
//	protocol_v3.go  ASSUMED  gramática de texto "*XX,imei,V1,...#" (família H02)
//	protocol_v4.go  UNKNOWN  esqueleto vazio, à espera de captura real
//
// A regra do projeto (§38) vale aqui sem exceção: campo não confirmado recebe
// "TODO: VERIFY AGAINST DEVICE PROTOCOL" e, na dúvida, o pacote é devolvido
// como KindOther com o payload cru — nunca interpretado "por semelhança".
//
// O caminho prático para sair do ASSUMED: ligue o aparelho apontando para o
// servidor, deixe os pacotes caírem em raw_packets e compare com o que está
// aqui. Ver docs/PROTOCOLS.md.
package tkstar

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/protocols"
)

// maxTextFrame limita o tamanho de uma linha antes de considerá-la lixo (§7).
const maxTextFrame = 1024

// textFramer separa quadros de texto delimitados por um terminador.
type textFramer struct {
	terminator byte
}

// nextFrame devolve o próximo quadro terminado pelo delimitador.
func (f textFramer) nextFrame(buf []byte) ([]byte, int, error) {
	// Descarta quebras de linha e espaços que separam quadros.
	skip := 0
	for skip < len(buf) && (buf[skip] == '\r' || buf[skip] == '\n' || buf[skip] == ' ') {
		skip++
	}
	if skip > 0 {
		return nil, skip, nil
	}

	idx := -1
	for i := range buf {
		if buf[i] == f.terminator {
			idx = i
			break
		}
	}
	if idx < 0 {
		if len(buf) > maxTextFrame {
			return nil, len(buf), fmt.Errorf("quadro de texto acima de %d bytes sem terminador", maxTextFrame)
		}
		return nil, 0, protocols.ErrIncompleteFrame
	}
	if idx+1 > maxTextFrame {
		return nil, idx + 1, fmt.Errorf("quadro de texto acima do limite")
	}
	return buf[:idx+1], idx + 1, nil
}

// field devolve o campo na posição indicada, ou vazio se não existir.
func field(parts []string, i int) string {
	if i < 0 || i >= len(parts) {
		return ""
	}
	return strings.TrimSpace(parts[i])
}

// parseFloatField lê um campo numérico opcional.
func parseFloatField(parts []string, i int) (float64, bool) {
	raw := field(parts, i)
	if raw == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// parseIntField lê um campo inteiro opcional.
func parseIntField(parts []string, i int) (int, bool) {
	raw := field(parts, i)
	if raw == "" {
		return 0, false
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return v, true
}

// extractIMEI lê "imei:<digitos>" ou apenas "<digitos>".
func extractIMEI(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "imei:")
	raw = strings.TrimPrefix(raw, "IMEI:")
	if err := protocols.ValidateIMEI(raw); err != nil {
		return "", err
	}
	return raw, nil
}
