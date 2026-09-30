package h02

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/protocols"
)

// Quadro de texto da família H02:
//
//	*HQ,869247061234567,V1,134530,A,2332.5000,S,04638.0000,W,000.00,000,170926,FFFFFBFF#
//	*HQ,869247061234567,V4,S20,134530,...#      resposta a um comando
//	*HQ,869247061234567,HTBT,87#                heartbeat
//
// A leitura é por posição fixa, não por expressão regular permissiva: linha
// que não bate exatamente com o formato volta como KindOther e vai inteira
// para a captura crua, em vez de ser interpretada "por aproximação" (§38).

// Índices dos campos do quadro de posição.
const (
	textFieldHeader = 0
	textFieldID     = 1
	textFieldKind   = 2
	textFieldTime   = 3
	textFieldValid  = 4
	textFieldLat    = 5
	textFieldLatHem = 6
	textFieldLon    = 7
	textFieldLonHem = 8
	textFieldSpeed  = 9
	textFieldCourse = 10
	textFieldDate   = 11
	textFieldStatus = 12
	textMinFields   = 12
)

func decodeText(frame []byte) (protocols.TrackerMessage, error) {
	raw := strings.TrimSpace(string(frame))
	line := strings.TrimSuffix(raw, "#")
	parts := strings.Split(line, ",")

	msg := protocols.TrackerMessage{
		Protocol:   Name,
		Kind:       protocols.KindOther,
		RawPayload: raw,
		Timestamp:  time.Now().UTC(),
		Attributes: map[string]any{
			"form":   "text",
			"header": field(parts, textFieldHeader),
		},
	}

	id := strings.TrimSpace(field(parts, textFieldID))
	if err := protocols.ValidateIMEI(id); err != nil {
		return msg, fmt.Errorf("quadro de texto H02: %w", err)
	}
	msg.IMEI = id

	kind := strings.ToUpper(field(parts, textFieldKind))
	msg.Attributes["messageType"] = kind

	switch {
	case kind == "HTBT":
		msg.Kind = protocols.KindHeartbeat
		if battery, ok := parseIntField(parts, 3); ok && battery >= 0 && battery <= 100 {
			msg.BatteryPercent = protocols.Ptr(battery)
		}
		return msg, nil

	case kind == "V4":
		// Resposta a um comando. O protocolo de texto não carrega chave de
		// correlação: o casamento é feito pelo comando aberto mais antigo.
		msg.Kind = protocols.KindCommandAck
		msg.Response = strings.Join(parts[3:], ",")
		if msg.Response == "" {
			msg.Response = field(parts, 3)
		}
		return msg, nil

	case strings.HasPrefix(kind, "V"):
		if err := decodeTextPosition(&msg, parts); err != nil {
			return msg, err
		}
		return msg, nil

	default:
		msg.Attributes["unrecognized"] = true
		return msg, nil
	}
}

func decodeTextPosition(msg *protocols.TrackerMessage, parts []string) error {
	if len(parts) < textMinFields {
		msg.Attributes["shortPacket"] = true
		return nil
	}

	msg.GPSValid = strings.ToUpper(field(parts, textFieldValid)) == "A"

	latitude, err := protocols.DDMMToDecimal(field(parts, textFieldLat), field(parts, textFieldLatHem))
	if err != nil {
		return fmt.Errorf("latitude: %w", err)
	}
	longitude, err := protocols.DDMMToDecimal(field(parts, textFieldLon), field(parts, textFieldLonHem))
	if err != nil {
		return fmt.Errorf("longitude: %w", err)
	}
	if !protocols.ValidCoordinates(latitude, longitude) {
		return fmt.Errorf("coordenadas fora de faixa: %f,%f", latitude, longitude)
	}
	msg.Latitude = latitude
	msg.Longitude = longitude
	msg.HasLocation = true
	msg.Kind = protocols.KindPosition

	// Mesma convenção do quadro binário: o campo é em nós.
	if knots, ok := parseFloatField(parts, textFieldSpeed); ok {
		msg.SpeedKmh = protocols.KnotsToKmh(knots)
		msg.Attributes["speedRaw"] = knots
		msg.Attributes["speedUnit"] = "knots"
	}
	if course, ok := parseFloatField(parts, textFieldCourse); ok {
		msg.Heading = course
	}

	if timestamp, err := parseTextTimestamp(
		field(parts, textFieldDate), field(parts, textFieldTime)); err == nil {
		msg.Timestamp = timestamp
	} else {
		msg.Attributes["timestampFallback"] = err.Error()
	}

	// A palavra de estado usa os mesmos bits do quadro binário.
	if status := strings.TrimSpace(field(parts, textFieldStatus)); status != "" {
		value, err := strconv.ParseUint(status, 16, 32)
		if err != nil {
			msg.Attributes["statusWord"] = status
			msg.Attributes["statusWordUnparsed"] = true
		} else {
			applyStatus(msg, uint32(value))
		}
	}

	return nil
}

// parseTextTimestamp junta ddMMyy + hhmmss em UTC.
func parseTextTimestamp(date, clock string) (time.Time, error) {
	if len(date) < 6 || len(clock) < 6 {
		return time.Time{}, fmt.Errorf("data/hora incompletas: %q %q", date, clock)
	}
	parsed, err := time.Parse("020106150405", date[:6]+clock[:6])
	if err != nil {
		return time.Time{}, fmt.Errorf("data/hora inválidas: %w", err)
	}
	return parsed.UTC(), nil
}

func field(parts []string, index int) string {
	if index < 0 || index >= len(parts) {
		return ""
	}
	return strings.TrimSpace(parts[index])
}

func parseFloatField(parts []string, index int) (float64, bool) {
	raw := field(parts, index)
	if raw == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

func parseIntField(parts []string, index int) (int, bool) {
	raw := field(parts, index)
	if raw == "" {
		return 0, false
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return value, true
}
