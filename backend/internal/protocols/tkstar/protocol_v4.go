package tkstar

import (
	"encoding/hex"
	"strings"
	"time"
	"unicode"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/protocols"
)

// NÍVEL DE CONFIANÇA: UNKNOWN
//
// Este arquivo é deliberadamente um esqueleto. Não há, no material disponível,
// descrição confiável do quadro "V4" dos TKSTAR 4G — e a regra §38 do projeto
// proíbe implementação "provável". Escrever um parser por semelhança aqui
// produziria latitude, velocidade e estado de ignição plausíveis e errados,
// que é o pior resultado possível num sistema que corta motor.
//
// COMO PREENCHER ESTE ARQUIVO
//
//  1. Aponte o aparelho para o servidor (comando SERVER/adminip do manual).
//  2. Deixe-o conectar. Como nenhum protocolo vai reconhecer o tráfego, cada
//     leitura cai em raw_packets com hexdump + ASCII.
//  3. Consulte:  GET /api/diagnostics/raw-packets
//     ou:        SELECT payload_hex, payload_ascii FROM raw_packets
//                ORDER BY received_at DESC LIMIT 50;
//  4. Se o payload for legível (começa com "*", "imei:", "$$"), o dialeto é de
//     texto: copie NewV1/NewV3 como ponto de partida.
//     Se for binário começando em 78 78 / 79 79, é GT06: use aquele adaptador.
//     Se for outra coisa, implemente Detect/NextFrame/Parse aqui, campo a campo,
//     conferindo cada um contra o manual — e só então mude Confidence.
//  5. Escreva o teste com os bytes REAIS capturados antes de ligar em produção.
//
// Enquanto Enabled for falso, este adaptador não reivindica tráfego nenhum:
// ele existe para capturar, documentar e dar lugar à implementação correta.

const NameV4 = "tkstar_v4"

// ProtocolV4 captura tráfego não identificado sem interpretá-lo.
type ProtocolV4 struct {
	// Enabled liga a captura por este adaptador (TKSTAR_V4_CAPTURE=true).
	// Mesmo ligado ele nunca produz posição: apenas registra o payload.
	Enabled bool
}

func NewV4(enabled bool) *ProtocolV4 { return &ProtocolV4{Enabled: enabled} }

func (p *ProtocolV4) Name() string { return NameV4 }

func (p *ProtocolV4) Describe() protocols.Descriptor {
	return protocols.Descriptor{
		Name:       NameV4,
		Label:      "TKSTAR V4 (não implementado — modo captura)",
		Vendor:     "TKSTAR",
		Confidence: protocols.Unknown,
		Commands:   nil,
		Notes: "Formato de quadro não confirmado. Nenhuma posição é produzida e " +
			"nenhum comando é aceito. Serve para capturar o tráfego real do " +
			"aparelho e, a partir dele, escrever o parser correto.",
	}
}

// Detect só aceita tráfego quando a captura está explicitamente ligada, e
// mesmo assim como último recurso do registro.
func (p *ProtocolV4) Detect(data []byte) bool { return p.Enabled && len(data) > 0 }

// NextFrame entrega o buffer inteiro: sem gramática conhecida, não há como
// saber onde um quadro termina.
func (p *ProtocolV4) NextFrame(buf []byte) ([]byte, int, error) {
	if len(buf) == 0 {
		return nil, 0, protocols.ErrIncompleteFrame
	}
	return buf, len(buf), nil
}

// Parse devolve o payload cru, sem qualquer interpretação.
func (p *ProtocolV4) Parse(frame []byte) ([]protocols.TrackerMessage, error) {
	return []protocols.TrackerMessage{{
		Protocol:   NameV4,
		Kind:       protocols.KindOther,
		Timestamp:  time.Now().UTC(),
		RawPayload: strings.ToUpper(hex.EncodeToString(frame)),
		Attributes: map[string]any{
			"capture": true,
			"ascii":   printableASCII(frame),
			"bytes":   len(frame),
		},
	}}, nil
}

// EncodeCommand recusa tudo: mandar bytes a um aparelho cujo protocolo não se
// conhece é exatamente o tipo de chute que a regra §38 proíbe.
func (p *ProtocolV4) EncodeCommand(protocols.Command) ([]byte, error) {
	return nil, protocols.ErrUnsupportedCommand
}

// printableASCII troca bytes não imprimíveis por ponto, para leitura humana.
func printableASCII(b []byte) string {
	var sb strings.Builder
	sb.Grow(len(b))
	for _, c := range b {
		if c < unicode.MaxASCII && unicode.IsPrint(rune(c)) {
			sb.WriteByte(c)
		} else {
			sb.WriteByte('.')
		}
	}
	return sb.String()
}
