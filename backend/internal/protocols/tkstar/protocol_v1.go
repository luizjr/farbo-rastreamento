package tkstar

import (
	"fmt"
	"strings"
	"time"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/protocols"
)

// NÍVEL DE CONFIANÇA: ASSUMED
//
// Esta variante implementa a gramática de texto terminada em ";" da família
// GPS103/TK103, que boa parte dos TKSTAR de geração anterior fala:
//
//	login      ##,imei:869247061234567,A;
//	heartbeat  869247061234567;
//	posição    imei:869247061234567,tracker,2509171345,,F,134530.000,A,
//	           2332.5000,S,04638.0000,W,0.00,0;
//
// TODO: VERIFY AGAINST DEVICE PROTOCOL — a ordem dos campos, o significado do
// campo de tipo e as respostas "LOAD"/"ON" precisam ser confirmados contra o
// firmware do seu aparelho. Enquanto isso, qualquer linha que não bata com o
// formato acima volta como KindOther e vai inteira para raw_packets.

const (
	NameV1         = "tkstar_v1"
	NameV1Extended = "tkstar_v1_8"
)

// Índices dos campos do pacote de posição. ASSUMED.
const (
	v1FieldIMEI      = 0
	v1FieldType      = 1
	v1FieldLocalTime = 2
	v1FieldPhone     = 3
	v1FieldFix       = 4
	v1FieldUTCTime   = 5
	v1FieldValid     = 6
	v1FieldLat       = 7
	v1FieldLatHemi   = 8
	v1FieldLon       = 9
	v1FieldLonHemi   = 10
	v1FieldSpeedKn   = 11
	v1FieldCourse    = 12
	v1FieldAltitude  = 13
	v1FieldBattery   = 14
	v1MinFields      = 12
)

// ProtocolV1 atende as variantes V1 e V1.8.
type ProtocolV1 struct {
	name     string
	label    string
	extended bool
	commands commandSet
	framer   textFramer
}

func NewV1() *ProtocolV1 {
	return &ProtocolV1{
		name:     NameV1,
		label:    "TKSTAR texto V1 (família GPS103)",
		commands: defaultCommands(),
		framer:   textFramer{terminator: ';'},
	}
}

// NewV1Extended trata a V1.8, que acrescenta altitude e bateria ao fim da linha.
func NewV1Extended() *ProtocolV1 {
	p := NewV1()
	p.name = NameV1Extended
	p.label = "TKSTAR texto V1.8 (família GPS103 estendida)"
	p.extended = true
	return p
}

func (p *ProtocolV1) Name() string { return p.name }

func (p *ProtocolV1) Describe() protocols.Descriptor {
	return protocols.Descriptor{
		Name:       p.name,
		Label:      p.label,
		Vendor:     "TKSTAR",
		Confidence: protocols.Assumed,
		Commands:   supportedCommands(p.commands),
		Notes: "Gramática inferida da família GPS103/TK103. Confirme contra o " +
			"manual do aparelho antes de operar o corte de motor. O protocolo " +
			"não carrega chave de correlação: o ACK é casado com o comando " +
			"pendente mais antigo.",
	}
}

func (p *ProtocolV1) Detect(data []byte) bool {
	s := strings.TrimLeft(string(data), "\r\n ")
	if s == "" {
		return false
	}
	if strings.HasPrefix(s, "##,imei:") || strings.HasPrefix(s, "imei:") {
		return true
	}
	// Heartbeat é o IMEI puro seguido de ";".
	if idx := strings.IndexByte(s, ';'); idx > 9 {
		return protocols.ValidateIMEI(s[:idx]) == nil
	}
	// Ainda pode estar chegando: aceita um prefixo só de dígitos.
	if len(s) <= 20 && isAllDigits(s) {
		return true
	}
	return false
}

func (p *ProtocolV1) NextFrame(buf []byte) ([]byte, int, error) {
	return p.framer.nextFrame(buf)
}

func (p *ProtocolV1) EncodeCommand(cmd protocols.Command) ([]byte, error) {
	return encodeCommand(p.commands, cmd)
}

func (p *ProtocolV1) EngineCut() []byte       { return []byte(p.commands.engineCut) }
func (p *ProtocolV1) EngineResume() []byte    { return []byte(p.commands.engineResume) }
func (p *ProtocolV1) RequestPosition() []byte { return []byte(p.commands.requestPosition) }
func (p *ProtocolV1) RequestStatus() []byte   { return []byte(p.commands.requestStatus) }

func (p *ProtocolV1) Parse(frame []byte) ([]protocols.TrackerMessage, error) {
	raw := strings.TrimSpace(string(frame))
	line := strings.TrimSuffix(raw, ";")

	msg := protocols.TrackerMessage{
		Protocol:   p.name,
		RawPayload: raw,
		Kind:       protocols.KindOther,
		Attributes: map[string]any{},
		Timestamp:  time.Now().UTC(),
	}

	switch {
	case strings.HasPrefix(line, "##,"):
		// Handshake de login: ##,imei:<imei>,A
		parts := strings.Split(line, ",")
		if len(parts) < 2 {
			return nil, fmt.Errorf("login TKSTAR V1 sem campo de IMEI")
		}
		imei, err := extractIMEI(parts[1])
		if err != nil {
			return nil, fmt.Errorf("login TKSTAR V1: %w", err)
		}
		msg.Kind = protocols.KindLogin
		msg.IMEI = imei
		msg.Ack = []byte("LOAD") // ASSUMED
		return []protocols.TrackerMessage{msg}, nil

	case isAllDigits(line):
		// Heartbeat: apenas o IMEI.
		imei, err := extractIMEI(line)
		if err != nil {
			return nil, fmt.Errorf("heartbeat TKSTAR V1: %w", err)
		}
		msg.Kind = protocols.KindHeartbeat
		msg.IMEI = imei
		msg.Ack = []byte("ON") // ASSUMED
		return []protocols.TrackerMessage{msg}, nil

	case strings.HasPrefix(line, "imei:"):
		if err := p.parseData(&msg, line); err != nil {
			return nil, err
		}
		return []protocols.TrackerMessage{msg}, nil

	default:
		// Não reconhecido: devolve cru, sem inventar interpretação (§38).
		msg.Attributes["unrecognized"] = true
		return []protocols.TrackerMessage{msg}, nil
	}
}

func (p *ProtocolV1) parseData(msg *protocols.TrackerMessage, line string) error {
	parts := strings.Split(line, ",")

	imei, err := extractIMEI(field(parts, v1FieldIMEI))
	if err != nil {
		return fmt.Errorf("pacote TKSTAR V1: %w", err)
	}
	msg.IMEI = imei

	kind := strings.ToLower(field(parts, v1FieldType))
	msg.Attributes["messageType"] = kind
	applyV1Type(msg, kind)

	if len(parts) < v1MinFields {
		// Pacote curto: é evento/estado sem coordenada. Não adivinhamos campos.
		msg.Attributes["shortPacket"] = true
		return nil
	}

	// "F" indica fix de GPS; "L" indica posicionamento por antena (sem GPS).
	fix := strings.ToUpper(field(parts, v1FieldFix))
	valid := strings.ToUpper(field(parts, v1FieldValid)) == "A"
	msg.GPSValid = fix == "F" && valid

	if fix != "F" {
		msg.Kind = protocols.KindStatus
		msg.Attributes["positioning"] = "lbs"
		return nil
	}

	lat, err := protocols.DDMMToDecimal(field(parts, v1FieldLat), field(parts, v1FieldLatHemi))
	if err != nil {
		return fmt.Errorf("latitude TKSTAR V1: %w", err)
	}
	lon, err := protocols.DDMMToDecimal(field(parts, v1FieldLon), field(parts, v1FieldLonHemi))
	if err != nil {
		return fmt.Errorf("longitude TKSTAR V1: %w", err)
	}
	if !protocols.ValidCoordinates(lat, lon) {
		return fmt.Errorf("coordenadas fora de faixa: %f,%f", lat, lon)
	}
	msg.Latitude = lat
	msg.Longitude = lon
	msg.HasLocation = true
	if msg.Kind == protocols.KindOther {
		msg.Kind = protocols.KindPosition
	}

	// O campo de velocidade vem em nós; o domínio só conhece km/h (§9).
	if knots, ok := parseFloatField(parts, v1FieldSpeedKn); ok {
		msg.SpeedKmh = protocols.KnotsToKmh(knots)
	}
	if course, ok := parseFloatField(parts, v1FieldCourse); ok {
		msg.Heading = course
	}

	ts, err := parseV1Timestamp(field(parts, v1FieldLocalTime), field(parts, v1FieldUTCTime))
	if err != nil {
		// Sem data confiável preferimos registrar o horário do servidor a
		// gravar um instante errado no histórico.
		msg.Attributes["timestampFallback"] = err.Error()
	} else {
		msg.Timestamp = ts
	}

	if p.extended {
		// TODO: VERIFY AGAINST DEVICE PROTOCOL — posição de altitude e bateria
		// na V1.8. Só preenchemos se os campos existirem e forem numéricos.
		if alt, ok := parseFloatField(parts, v1FieldAltitude); ok {
			msg.Altitude = protocols.Ptr(alt)
		}
		if batt, ok := parseIntField(parts, v1FieldBattery); ok && batt >= 0 && batt <= 100 {
			msg.BatteryPercent = protocols.Ptr(batt)
		}
	}
	return nil
}

// applyV1Type traduz o campo de tipo/alarme. ASSUMED.
func applyV1Type(msg *protocols.TrackerMessage, kind string) {
	switch kind {
	case "tracker":
		// posição periódica normal
	case "acc on":
		msg.ACC = protocols.Ptr(true)
	case "acc off":
		msg.ACC = protocols.Ptr(false)
	case "help me":
		msg.Alarm = "SOS"
		msg.Kind = protocols.KindAlarm
	case "low battery":
		msg.Alarm = "LOW_BATTERY"
		msg.Kind = protocols.KindAlarm
	case "move", "stockade":
		msg.Alarm = "VIBRATION"
		msg.Kind = protocols.KindAlarm
	case "speed":
		msg.Alarm = "OVERSPEED"
		msg.Kind = protocols.KindAlarm
	default:
		// TODO: VERIFY AGAINST DEVICE PROTOCOL — demais rótulos de tipo.
		if kind != "" {
			msg.Attributes["unmappedType"] = kind
		}
	}
}

// parseV1Timestamp junta a data (yyMMddHHmm, hora local do aparelho) com o
// horário UTC (hhmmss.sss) do mesmo pacote. ASSUMED.
func parseV1Timestamp(localDateTime, utcTime string) (time.Time, error) {
	if len(localDateTime) < 10 {
		return time.Time{}, fmt.Errorf("campo de data curto: %q", localDateTime)
	}
	date, err := time.Parse("0601021504", localDateTime[:10])
	if err != nil {
		return time.Time{}, fmt.Errorf("data inválida %q: %w", localDateTime, err)
	}

	if len(utcTime) < 6 {
		return date.UTC(), nil
	}
	clock, err := time.Parse("150405", utcTime[:6])
	if err != nil {
		return time.Time{}, fmt.Errorf("hora inválida %q: %w", utcTime, err)
	}

	return time.Date(date.Year(), date.Month(), date.Day(),
		clock.Hour(), clock.Minute(), clock.Second(), 0, time.UTC), nil
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
