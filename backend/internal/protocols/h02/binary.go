package h02

import (
	"fmt"
	"time"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/protocols"
)

// ---------------------------------------------------------------------------
// Leitura BCD
//
// O quadro binário do H02 empacota números em BCD, às vezes com um dígito
// ímpar que divide o byte com o campo seguinte (o dígito fica no nibble alto,
// o campo seguinte usa o nibble baixo). O cursor abaixo reproduz exatamente
// essa mecânica.
//
// Diferença proposital em relação à implementação de referência: aqui todo
// nibble que deveria ser um dígito é validado. Quadro mal enquadrado gera erro
// e vai para a captura crua, em vez de virar uma posição plausível e errada —
// que é justamente o sintoma clássico deste protocolo (posições saltando para
// o meio do oceano).
// ---------------------------------------------------------------------------

type cursor struct {
	data []byte
	pos  int
}

func (c *cursor) remaining() int { return len(c.data) - c.pos }

func (c *cursor) readByte() (byte, error) {
	if c.pos >= len(c.data) {
		return 0, fmt.Errorf("fim inesperado do quadro no byte %d", c.pos)
	}
	b := c.data[c.pos]
	c.pos++
	return b, nil
}

func (c *cursor) peekByte() (byte, error) {
	if c.pos >= len(c.data) {
		return 0, fmt.Errorf("fim inesperado do quadro no byte %d", c.pos)
	}
	return c.data[c.pos], nil
}

// readBCD lê a quantidade de dígitos informada.
//
// Com número ímpar de dígitos, o último sai do nibble alto do próximo byte
// SEM consumi-lo: o nibble baixo pertence ao campo seguinte.
func (c *cursor) readBCD(digits int) (int, error) {
	result := 0

	for range digits / 2 {
		b, err := c.readByte()
		if err != nil {
			return 0, err
		}
		high, low := b>>4, b&0x0F
		if high > 9 || low > 9 {
			return 0, fmt.Errorf("byte BCD inválido %02X na posição %d", b, c.pos-1)
		}
		result = result*10 + int(high)
		result = result*10 + int(low)
	}

	if digits%2 != 0 {
		b, err := c.peekByte()
		if err != nil {
			return 0, err
		}
		high := b >> 4
		if high > 9 {
			return 0, fmt.Errorf("nibble BCD inválido %X na posição %d", high, c.pos)
		}
		result = result*10 + int(high)
	}

	return result, nil
}

// readUint32 lê a palavra de estado de 4 bytes.
func (c *cursor) readUint32() (uint32, error) {
	if c.remaining() < 4 {
		return 0, fmt.Errorf("faltam bytes para a palavra de estado")
	}
	value := uint32(c.data[c.pos])<<24 | uint32(c.data[c.pos+1])<<16 |
		uint32(c.data[c.pos+2])<<8 | uint32(c.data[c.pos+3])
	c.pos += 4
	return value, nil
}

// ---------------------------------------------------------------------------
// Campos
// ---------------------------------------------------------------------------

// minBinaryPayload é o mínimo decodificável: marcador + id curto (5) +
// data/hora (6) + latitude (4) + bateria (1) + longitude (4) + flags (1) +
// velocidade (1) + rumo (2) + estado (4).
const minBinaryPayload = 29

// binaryIDLong é o tamanho do identificador na variante de id longo.
const (
	binaryIDShort = 5
	binaryIDLong  = 8
)

// readCoordinate decodifica uma coordenada no formato do quadro binário.
//
// Latitude: GG (1 byte) + MM.MMMM (3 bytes).
// Longitude: GGG (1 byte + nibble alto do seguinte) + MM.MMMM, com o nibble
// baixo daquele byte fazendo parte dos minutos.
func (c *cursor) readCoordinate(isLongitude bool) (float64, error) {
	degrees, err := c.readBCD(2)
	if err != nil {
		return 0, err
	}

	minutes := 0.0

	if isLongitude {
		// O terceiro dígito dos graus vem do nibble alto do próximo byte,
		// que não é consumido aqui.
		next, err := c.peekByte()
		if err != nil {
			return 0, err
		}
		high := next >> 4
		if high > 9 {
			return 0, fmt.Errorf("nibble de grau inválido %X", high)
		}
		degrees = degrees*10 + int(high)

		// Agora o byte é consumido: o nibble baixo é o primeiro dígito dos minutos.
		b, err := c.readByte()
		if err != nil {
			return 0, err
		}
		low := b & 0x0F
		if low > 9 {
			return 0, fmt.Errorf("nibble de minuto inválido %X", low)
		}
		minutes = float64(low)
	}

	fractionDigits := 6
	if isLongitude {
		fractionDigits = 5
	}
	fraction, err := c.readBCD(fractionDigits)
	if err != nil {
		return 0, err
	}

	minutes = minutes*10 + float64(fraction)/10000.0
	if minutes >= 60 {
		return 0, fmt.Errorf("minutos fora de faixa: %f", minutes)
	}

	return float64(degrees) + minutes/60, nil
}

// decodeBattery traduz o byte de bateria. As faixas vêm da implementação de
// referência; valor fora delas devolve nil em vez de um número inventado.
func decodeBattery(value byte) *int {
	switch {
	case value == 0:
		return nil
	case value <= 3:
		return protocols.Ptr((int(value) - 1) * 10)
	case value <= 6:
		return protocols.Ptr((int(value) - 1) * 20)
	case value <= 100:
		return protocols.Ptr(int(value))
	case value >= 0xF1 && value <= 0xF6:
		return protocols.Ptr(int(value) - 0xF0)
	default:
		return nil
	}
}

// Bits da palavra de estado. Atenção: a lógica é invertida — o bit LIMPO
// indica alarme.
const (
	statusBitVibration = 0
	statusBitSOS       = 1
	statusBitOverspeed = 2
	statusBitIgnition  = 10
	statusBitSOSAlt    = 18
	statusBitPowerCut  = 19
)

func bitSet(status uint32, bit uint) bool { return status&(1<<bit) != 0 }

// applyStatus traduz a palavra de estado de 4 bytes, comum ao quadro binário
// e à última coluna do quadro de texto.
func applyStatus(msg *protocols.TrackerMessage, status uint32) {
	msg.Attributes["statusWord"] = fmt.Sprintf("%08X", status)

	switch {
	case !bitSet(status, statusBitVibration):
		msg.Alarm = "VIBRATION"
	case !bitSet(status, statusBitSOS) || !bitSet(status, statusBitSOSAlt):
		msg.Alarm = "SOS"
	case !bitSet(status, statusBitOverspeed):
		msg.Alarm = "OVERSPEED"
	case !bitSet(status, statusBitPowerCut):
		msg.Alarm = "POWER_LOSS"
	}

	msg.ACC = protocols.Ptr(bitSet(status, statusBitIgnition))

	if msg.Alarm != "" {
		msg.Kind = protocols.KindAlarm
	}
}

// decodeBinary interpreta um quadro que começa com '$'.
func decodeBinary(frame []byte) (protocols.TrackerMessage, error) {
	msg := protocols.TrackerMessage{
		Protocol:   Name,
		Kind:       protocols.KindPosition,
		RawPayload: hexdump(frame),
		Attributes: map[string]any{"form": "binary", "frameBytes": len(frame)},
	}

	if len(frame) < minBinaryPayload {
		return msg, fmt.Errorf("quadro binário curto: %d bytes, mínimo %d", len(frame), minBinaryPayload)
	}

	c := &cursor{data: frame}
	if _, err := c.readByte(); err != nil { // marcador '$'
		return msg, err
	}

	// A variante de id longo usa 8 bytes; a comum, 5.
	idLength := binaryIDShort
	if len(frame) == 42 {
		idLength = binaryIDLong
	}
	if c.remaining() < idLength {
		return msg, fmt.Errorf("quadro sem identificador completo")
	}
	msg.IMEI = normalizeID(frame[c.pos : c.pos+idLength])
	c.pos += idLength

	if err := protocols.ValidateIMEI(msg.IMEI); err != nil {
		return msg, fmt.Errorf("identificador do quadro binário: %w", err)
	}

	timestamp, err := readBinaryTime(c)
	if err != nil {
		return msg, err
	}
	msg.Timestamp = timestamp

	latitude, err := c.readCoordinate(false)
	if err != nil {
		return msg, fmt.Errorf("latitude: %w", err)
	}

	batteryByte, err := c.readByte()
	if err != nil {
		return msg, err
	}
	msg.BatteryPercent = decodeBattery(batteryByte)

	longitude, err := c.readCoordinate(true)
	if err != nil {
		return msg, fmt.Errorf("longitude: %w", err)
	}

	flagsByte, err := c.readByte()
	if err != nil {
		return msg, err
	}
	flags := flagsByte & 0x0F

	msg.GPSValid = flags&0x02 != 0
	if flags&0x04 == 0 {
		latitude = -latitude
	}
	if flags&0x08 == 0 {
		longitude = -longitude
	}
	if !protocols.ValidCoordinates(latitude, longitude) {
		return msg, fmt.Errorf("coordenadas fora de faixa: %f,%f", latitude, longitude)
	}
	msg.Latitude = latitude
	msg.Longitude = longitude
	msg.HasLocation = true

	speed, err := c.readBCD(3)
	if err != nil {
		return msg, fmt.Errorf("velocidade: %w", err)
	}
	// A implementação de referência trata este campo como NÓS, como manda a
	// herança NMEA da família. O valor cru fica preservado para conferência.
	//
	// TODO: VERIFY AGAINST DEVICE PROTOCOL — confirme com o aparelho em
	// movimento. Um erro aqui é 1,852x na velocidade, e a velocidade decide
	// se o corte de motor é autorizado.
	msg.SpeedKmh = protocols.KnotsToKmh(float64(speed))
	msg.Attributes["speedRaw"] = speed
	msg.Attributes["speedUnit"] = "knots"

	courseHigh, err := c.readByte()
	if err != nil {
		return msg, err
	}
	courseLow, err := c.readBCD(2)
	if err != nil {
		return msg, fmt.Errorf("rumo: %w", err)
	}
	course := float64(courseHigh&0x0F)*100 + float64(courseLow)
	if course > 360 {
		return msg, fmt.Errorf("rumo fora de faixa: %f", course)
	}
	msg.Heading = course

	status, err := c.readUint32()
	if err != nil {
		return msg, err
	}
	applyStatus(&msg, status)

	// O que sobra depois do bloco conhecido fica guardado cru: quadros mais
	// longos trazem dados extras que ainda não foram confirmados.
	if c.remaining() > 0 {
		msg.Attributes["trailing"] = hexdump(frame[c.pos:])
	}

	return msg, nil
}

// readBinaryTime lê hora, minuto, segundo, dia, mês e ano, todos em BCD.
func readBinaryTime(c *cursor) (time.Time, error) {
	fields := make([]int, 6)
	for i := range fields {
		value, err := c.readBCD(2)
		if err != nil {
			return time.Time{}, fmt.Errorf("data/hora: %w", err)
		}
		fields[i] = value
	}
	hour, minute, second, day, month, year := fields[0], fields[1], fields[2], fields[3], fields[4], fields[5]

	if hour > 23 || minute > 59 || second > 59 || day < 1 || day > 31 || month < 1 || month > 12 {
		return time.Time{}, fmt.Errorf(
			"data/hora implausível: %02d:%02d:%02d %02d/%02d/%02d", hour, minute, second, day, month, year)
	}

	return time.Date(2000+year, time.Month(month), day, hour, minute, second, 0, time.UTC), nil
}

// normalizeID transforma os bytes de identificador em dígitos decimais.
// Cada byte carrega dois dígitos BCD, então o hexadecimal já é a leitura.
func normalizeID(raw []byte) string {
	out := make([]byte, 0, len(raw)*2)
	const digits = "0123456789abcdef"
	for _, b := range raw {
		out = append(out, digits[b>>4], digits[b&0x0F])
	}
	return string(out)
}
