package tkstar

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/protocols"
)

// ATENÇÃO: as linhas usadas aqui seguem a gramática ASSUMED descrita no topo
// de cada arquivo de variante. Elas fixam o comportamento do parser, não a
// verdade sobre o firmware.
//
// TODO: VERIFY AGAINST DEVICE PROTOCOL — substituir por capturas reais de um
// TK910/TK970 assim que houver aparelho em mãos.

const (
	v1Position  = "imei:869247061234567,tracker,2509171345,,F,134530.000,A,2332.5000,S,04638.0000,W,0.00,0;"
	v1Heartbeat = "869247061234567;"
	v1Login     = "##,imei:869247061234567,A;"
	v3Position  = "*HQ,869247061234567,V1,134530,A,2332.5000,S,04638.0000,W,000.00,000,170926,FFFFFBFF#"
)

func parseOne(t *testing.T, p protocols.TrackerProtocol, frame string) protocols.TrackerMessage {
	t.Helper()
	msgs, err := p.Parse([]byte(frame))
	if err != nil {
		t.Fatalf("Parse falhou: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("esperava 1 mensagem, recebi %d", len(msgs))
	}
	return msgs[0]
}

// ---------------------------------------------------------------------------
// V1
// ---------------------------------------------------------------------------

func TestV1ParsePosition(t *testing.T) {
	msg := parseOne(t, NewV1(), v1Position)

	if msg.Kind != protocols.KindPosition {
		t.Fatalf("esperava posição, recebi %q", msg.Kind)
	}
	if msg.IMEI != "869247061234567" {
		t.Fatalf("IMEI incorreto: %q", msg.IMEI)
	}
	if !msg.GPSValid {
		t.Fatal("esperava fix válido")
	}
	if !msg.HasLocation {
		t.Fatal("esperava coordenada")
	}

	wantLat := -(23 + 32.5/60)
	wantLon := -(46 + 38.0/60)
	if math.Abs(msg.Latitude-wantLat) > 1e-9 {
		t.Fatalf("latitude: esperava %f, recebi %f", wantLat, msg.Latitude)
	}
	if math.Abs(msg.Longitude-wantLon) > 1e-9 {
		t.Fatalf("longitude: esperava %f, recebi %f", wantLon, msg.Longitude)
	}

	want := time.Date(2025, 9, 17, 13, 45, 30, 0, time.UTC)
	if !msg.Timestamp.Equal(want) {
		t.Fatalf("timestamp: esperava %s, recebi %s", want, msg.Timestamp)
	}
	if msg.RawPayload != v1Position {
		t.Fatal("RawPayload deveria preservar a linha original")
	}
}

func TestV1ParseSpeedConvertsKnots(t *testing.T) {
	line := strings.Replace(v1Position, ",0.00,0;", ",54.00,270;", 1)
	msg := parseOne(t, NewV1(), line)

	// 54 nós = 100,008 km/h. O domínio nunca vê nós (§9).
	if math.Abs(msg.SpeedKmh-100.008) > 1e-9 {
		t.Fatalf("esperava 100.008 km/h, recebi %v", msg.SpeedKmh)
	}
	if msg.Heading != 270 {
		t.Fatalf("heading incorreto: %v", msg.Heading)
	}
}

func TestV1ParseACC(t *testing.T) {
	on := strings.Replace(v1Position, ",tracker,", ",acc on,", 1)
	if msg := parseOne(t, NewV1(), on); msg.ACC == nil || !*msg.ACC {
		t.Fatalf("esperava ACC ligado, recebi %v", msg.ACC)
	}

	off := strings.Replace(v1Position, ",tracker,", ",acc off,", 1)
	if msg := parseOne(t, NewV1(), off); msg.ACC == nil || *msg.ACC {
		t.Fatalf("esperava ACC desligado, recebi %v", msg.ACC)
	}

	// Posição comum não afirma nada sobre a ignição.
	if msg := parseOne(t, NewV1(), v1Position); msg.ACC != nil {
		t.Fatalf("ACC deveria ficar indefinido, recebi %v", *msg.ACC)
	}
}

func TestV1ParseAlarm(t *testing.T) {
	line := strings.Replace(v1Position, ",tracker,", ",help me,", 1)
	msg := parseOne(t, NewV1(), line)

	if msg.Kind != protocols.KindAlarm || msg.Alarm != "SOS" {
		t.Fatalf("esperava alarme SOS, recebi %q/%q", msg.Kind, msg.Alarm)
	}
	// Mesmo sendo alarme, a coordenada do pacote continua sendo aproveitada.
	if !msg.HasLocation {
		t.Fatal("alarme com fix deveria trazer coordenada")
	}
}

func TestV1ParseHeartbeatAndLogin(t *testing.T) {
	hb := parseOne(t, NewV1(), v1Heartbeat)
	if hb.Kind != protocols.KindHeartbeat || hb.IMEI != "869247061234567" {
		t.Fatalf("heartbeat inesperado: %q / %q", hb.Kind, hb.IMEI)
	}
	if string(hb.Ack) != "ON" {
		t.Fatalf("ACK de heartbeat: %q", hb.Ack)
	}
	if hb.HasLocation {
		t.Fatal("heartbeat não tem coordenada")
	}

	login := parseOne(t, NewV1(), v1Login)
	if login.Kind != protocols.KindLogin || login.IMEI != "869247061234567" {
		t.Fatalf("login inesperado: %q / %q", login.Kind, login.IMEI)
	}
	if string(login.Ack) != "LOAD" {
		t.Fatalf("ACK de login: %q", login.Ack)
	}
}

func TestV1ParseLBSPacketHasNoCoordinates(t *testing.T) {
	// "L" no lugar de "F" indica posicionamento por antena, sem GPS.
	line := strings.Replace(v1Position, ",,F,", ",,L,", 1)
	msg := parseOne(t, NewV1(), line)

	if msg.HasLocation {
		t.Fatal("pacote sem fix não pode virar posição")
	}
	if msg.GPSValid {
		t.Fatal("GPSValid deveria ser falso")
	}
	if msg.Kind != protocols.KindStatus {
		t.Fatalf("esperava status, recebi %q", msg.Kind)
	}
}

func TestV1ParseInvalid(t *testing.T) {
	bad := []string{
		"imei:123,tracker,2509171345,,F,134530.000,A,2332.5000,S,04638.0000,W,0.00,0;",             // IMEI curto
		"imei:869247061234567,tracker,2509171345,,F,134530.000,A,9999.9999,S,04638.0000,W,0.00,0;", // minutos inválidos
		"##,;",
	}
	for _, line := range bad {
		if _, err := NewV1().Parse([]byte(line)); err == nil {
			t.Fatalf("esperava erro para %q", line)
		}
	}
}

func TestV1UnknownLineIsCapturedNotGuessed(t *testing.T) {
	msg := parseOne(t, NewV1(), "algo totalmente inesperado;")

	if msg.Kind != protocols.KindOther {
		t.Fatalf("linha desconhecida não pode virar %q", msg.Kind)
	}
	if msg.HasLocation {
		t.Fatal("linha desconhecida não pode produzir coordenada")
	}
	if msg.Attributes["unrecognized"] != true {
		t.Fatal("linha desconhecida deveria ser marcada para análise")
	}
}

func TestV1Detect(t *testing.T) {
	p := NewV1()
	for _, ok := range []string{v1Position, v1Heartbeat, v1Login, "8692470"} {
		if !p.Detect([]byte(ok)) {
			t.Fatalf("deveria reconhecer %q", ok)
		}
	}
	for _, no := range []string{"*HQ,869247061234567,V1,", string([]byte{0x78, 0x78, 0x0D})} {
		if p.Detect([]byte(no)) {
			t.Fatalf("não deveria reconhecer %q", no)
		}
	}
}

// ---------------------------------------------------------------------------
// Enquadramento de texto: 1 Read() != 1 pacote (§31)
// ---------------------------------------------------------------------------

func TestV1FramingSplitAndMultiple(t *testing.T) {
	p := NewV1()
	stream := v1Login + v1Heartbeat + v1Position

	var buf []byte
	var frames []string

	// Entrega o fluxo em pedaços de 7 bytes.
	for i := 0; i < len(stream); i += 7 {
		end := min(i+7, len(stream))
		buf = append(buf, stream[i:end]...)

		for {
			frame, consumed, err := p.NextFrame(buf)
			if errors.Is(err, protocols.ErrIncompleteFrame) {
				break
			}
			if err != nil {
				t.Fatalf("erro inesperado: %v", err)
			}
			buf = buf[consumed:]
			if frame != nil {
				frames = append(frames, string(frame))
			}
		}
	}

	want := []string{v1Login, v1Heartbeat, v1Position}
	if len(frames) != len(want) {
		t.Fatalf("esperava %d quadros, recebi %d: %q", len(want), len(frames), frames)
	}
	for i := range want {
		if frames[i] != want[i] {
			t.Fatalf("quadro %d: esperava %q, recebi %q", i, want[i], frames[i])
		}
	}
	if len(buf) != 0 {
		t.Fatalf("sobraram %d bytes", len(buf))
	}
}

func TestV1FramingRejectsUnterminatedFlood(t *testing.T) {
	p := NewV1()
	flood := make([]byte, maxTextFrame+10)
	for i := range flood {
		flood[i] = 'A'
	}
	_, consumed, err := p.NextFrame(flood)
	if err == nil {
		t.Fatal("esperava recusa de quadro sem terminador")
	}
	if consumed != len(flood) {
		t.Fatalf("deveria descartar o buffer inteiro, descartou %d", consumed)
	}
}

// ---------------------------------------------------------------------------
// Comandos
// ---------------------------------------------------------------------------

func TestEncodeCommands(t *testing.T) {
	p := NewV1()

	tests := []struct {
		cmdType protocols.CommandType
		want    string
	}{
		{protocols.CommandEngineCut, "RELAY,1#"},
		{protocols.CommandEngineResume, "RELAY,0#"},
		{protocols.CommandRequestPosition, "WHERE#"},
		{protocols.CommandRequestStatus, "STATUS#"},
	}

	for _, tc := range tests {
		got, err := p.EncodeCommand(protocols.Command{Type: tc.cmdType})
		if err != nil {
			t.Fatalf("%s: %v", tc.cmdType, err)
		}
		if string(got) != tc.want {
			t.Fatalf("%s: esperava %q, recebi %q", tc.cmdType, tc.want, got)
		}
	}
}

func TestCommandHelpersMatchEncoder(t *testing.T) {
	p := NewV1()
	if string(p.EngineCut()) != "RELAY,1#" || string(p.EngineResume()) != "RELAY,0#" {
		t.Fatal("helpers de comando divergem do vocabulário")
	}
	if string(p.RequestPosition()) != "WHERE#" || string(p.RequestStatus()) != "STATUS#" {
		t.Fatal("helpers de consulta divergem do vocabulário")
	}
}

func TestEncodeUnconfirmedCommandIsRefused(t *testing.T) {
	p := NewV1()
	// Intervalo, heartbeat, servidor e reboot não têm texto confirmado para
	// esta variante: preferimos recusar a chutar bytes (§38).
	for _, cmdType := range []protocols.CommandType{
		protocols.CommandSetInterval, protocols.CommandSetHeartbeat,
		protocols.CommandSetServer, protocols.CommandReboot,
	} {
		_, err := p.EncodeCommand(protocols.Command{
			Type:   cmdType,
			Params: map[string]string{"seconds": "60", "minutes": "5", "host": "x.com", "port": "5000"},
		})
		if !errors.Is(err, protocols.ErrUnsupportedCommand) {
			t.Fatalf("%s: esperava ErrUnsupportedCommand, recebi %v", cmdType, err)
		}
	}
}

func TestEncodeCommandOverrideAndInjection(t *testing.T) {
	p := NewV1()

	got, err := p.EncodeCommand(protocols.Command{Type: protocols.CommandEngineCut, Raw: "DYD#"})
	if err != nil || string(got) != "DYD#" {
		t.Fatalf("override deveria prevalecer: %q %v", got, err)
	}

	if _, err := p.EncodeCommand(protocols.Command{
		Type: protocols.CommandEngineCut, Raw: "RELAY,1\n\rSTATUS#",
	}); err == nil {
		t.Fatal("texto com caractere de controle deveria ser recusado")
	}

	if _, err := p.EncodeCommand(protocols.Command{
		Type: protocols.CommandEngineCut, Password: "12;RESET",
	}); err == nil {
		t.Fatal("senha malformada deveria ser recusada")
	}
}

// ---------------------------------------------------------------------------
// V4 (UNKNOWN)
// ---------------------------------------------------------------------------

func TestV4NeverClaimsTrafficWhenDisabled(t *testing.T) {
	p := NewV4(false)
	if p.Detect([]byte("qualquer coisa")) {
		t.Fatal("V4 desligado não pode reivindicar tráfego")
	}
	if p.Describe().Confidence != protocols.Unknown {
		t.Fatal("V4 precisa se declarar UNKNOWN")
	}
}

func TestV4CapturesWithoutInterpreting(t *testing.T) {
	p := NewV4(true)
	payload := []byte{0x24, 0x24, 0x00, 0x1F, 'O', 'K'}

	msgs, err := p.Parse(payload)
	if err != nil {
		t.Fatalf("Parse falhou: %v", err)
	}
	msg := msgs[0]

	if msg.Kind != protocols.KindOther {
		t.Fatalf("captura não pode virar %q", msg.Kind)
	}
	if msg.HasLocation || msg.GPSValid || msg.ACC != nil {
		t.Fatal("captura não pode afirmar telemetria")
	}
	if msg.RawPayload != "2424001F4F4B" {
		t.Fatalf("hexdump incorreto: %q", msg.RawPayload)
	}
	if msg.Attributes["ascii"] != "$$..OK" {
		t.Fatalf("ascii incorreto: %v", msg.Attributes["ascii"])
	}
}

func TestV4RefusesCommands(t *testing.T) {
	if _, err := NewV4(true).EncodeCommand(protocols.Command{
		Type: protocols.CommandEngineCut,
	}); !errors.Is(err, protocols.ErrUnsupportedCommand) {
		t.Fatalf("esperava recusa, recebi %v", err)
	}
}
