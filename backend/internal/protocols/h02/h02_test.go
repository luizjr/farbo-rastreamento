package h02

import (
	"encoding/hex"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/protocols"
)

// ---------------------------------------------------------------------------
// Fixture principal: CAPTURA REAL
//
// Quadro binário de um TK905 4G, publicado no fórum do Traccar por quem tinha
// o aparelho em mãos. Não é sintético.
//
// Ele decodifica para uma coordenada real e coerente — Melbourne, parado,
// ignição desligada — o que é a evidência de que a ordem e o significado dos
// campos estão certos.
//
// TODO: VERIFY AGAINST DEVICE PROTOCOL — acrescentar aqui uma captura de
// TK915/TK910 em MOVIMENTO, que é o que falta para confirmar a unidade do
// campo de velocidade.
// ---------------------------------------------------------------------------

const realBinarySample = "2459051018930314371011213757325002145037076a00" +
	"0344ff7ffbffff00085b0000000001f90300000000640003"

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("hex inválido: %v", err)
	}
	return b
}

func parseOne(t *testing.T, p *Protocol, frame []byte) protocols.TrackerMessage {
	t.Helper()
	msgs, err := p.Parse(frame)
	if err != nil {
		t.Fatalf("Parse falhou: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("esperava 1 mensagem, recebi %d", len(msgs))
	}
	return msgs[0]
}

func newProtocol() *Protocol { return New(0) }

// ---------------------------------------------------------------------------
// Quadro binário
// ---------------------------------------------------------------------------

func TestDecodeRealBinaryCapture(t *testing.T) {
	frame := mustHex(t, realBinarySample)
	msg := parseOne(t, newProtocol(), frame)

	if msg.Kind != protocols.KindPosition {
		t.Fatalf("esperava posição, recebi %q", msg.Kind)
	}
	if msg.IMEI != "5905101893" {
		t.Fatalf("identificador incorreto: %q", msg.IMEI)
	}

	// 37°57.3250' S  →  -37.955417
	wantLat := -(37 + 57.3250/60)
	if math.Abs(msg.Latitude-wantLat) > 1e-9 {
		t.Fatalf("latitude: esperava %.9f, recebi %.9f", wantLat, msg.Latitude)
	}

	// 145°03.7076' E → 145.061793
	wantLon := 145 + 3.7076/60
	if math.Abs(msg.Longitude-wantLon) > 1e-9 {
		t.Fatalf("longitude: esperava %.9f, recebi %.9f", wantLon, msg.Longitude)
	}

	wantTime := time.Date(2021, 11, 10, 3, 14, 37, 0, time.UTC)
	if !msg.Timestamp.Equal(wantTime) {
		t.Fatalf("timestamp: esperava %s, recebi %s", wantTime, msg.Timestamp)
	}

	if !msg.GPSValid {
		t.Fatal("o bit de validade indica fix válido")
	}
	if msg.SpeedKmh != 0 {
		t.Fatalf("velocidade: esperava 0, recebi %v", msg.SpeedKmh)
	}
	if msg.Heading != 344 {
		t.Fatalf("rumo: esperava 344, recebi %v", msg.Heading)
	}

	// Palavra de estado 0xFF7FFBFF: bit 10 limpo = ignição desligada,
	// nenhum bit de alarme limpo.
	if msg.ACC == nil || *msg.ACC {
		t.Fatalf("ignição: esperava desligada, recebi %v", msg.ACC)
	}
	if msg.Alarm != "" {
		t.Fatalf("não deveria haver alarme, recebi %q", msg.Alarm)
	}
	if msg.Attributes["statusWord"] != "FF7FFBFF" {
		t.Fatalf("palavra de estado: %v", msg.Attributes["statusWord"])
	}

	// Byte de bateria 0x02 → faixa baixa da tabela de níveis.
	if msg.BatteryPercent == nil || *msg.BatteryPercent != 10 {
		t.Fatalf("bateria: %v", msg.BatteryPercent)
	}

	// O bloco conhecido tem 29 bytes; o resto é preservado cru.
	if msg.Attributes["frameBytes"] != 47 {
		t.Fatalf("tamanho do quadro: %v", msg.Attributes["frameBytes"])
	}
	if msg.Attributes["trailing"] != "FF00085B0000000001F90300000000640003" {
		t.Fatalf("sobra do quadro: %v", msg.Attributes["trailing"])
	}
	if msg.RawPayload != strings.ToUpper(realBinarySample) {
		t.Fatal("RawPayload deveria preservar o quadro original")
	}
}

func TestBinaryRejectsInvalidBCD(t *testing.T) {
	frame := mustHex(t, realBinarySample)

	// Estraga o byte dos minutos com um nibble que não é dígito decimal.
	corrupted := append([]byte(nil), frame...)
	corrupted[7] = 0xAB

	if _, err := newProtocol().Parse(corrupted); err == nil {
		t.Fatal("nibble BCD inválido deveria ser recusado, não interpretado")
	}
}

func TestBinaryRejectsImplausibleDate(t *testing.T) {
	frame := mustHex(t, realBinarySample)
	corrupted := append([]byte(nil), frame...)
	corrupted[10] = 0x13 // mês 13

	_, err := newProtocol().Parse(corrupted)
	if err == nil {
		t.Fatal("mês fora de faixa deveria ser recusado")
	}
	if !strings.Contains(err.Error(), "implausível") {
		t.Fatalf("erro inesperado: %v", err)
	}
}

func TestBinaryRejectsOutOfRangeMinutes(t *testing.T) {
	frame := mustHex(t, realBinarySample)
	corrupted := append([]byte(nil), frame...)
	corrupted[13] = 0x61 // 61 minutos de arco na latitude

	if _, err := newProtocol().Parse(corrupted); err == nil {
		t.Fatal("minutos fora de faixa deveriam ser recusados")
	}
}

func TestBinaryRejectsShortFrame(t *testing.T) {
	frame := mustHex(t, realBinarySample)[:20]

	if _, err := newProtocol().Parse(frame); err == nil {
		t.Fatal("quadro curto deveria ser recusado")
	}
}

func TestBinaryRejectsInvalidIdentifier(t *testing.T) {
	frame := mustHex(t, realBinarySample)
	corrupted := append([]byte(nil), frame...)
	// Nibble 'F' torna o identificador não numérico.
	corrupted[1] = 0xFF

	if _, err := newProtocol().Parse(corrupted); err == nil {
		t.Fatal("identificador não numérico deveria ser recusado")
	}
}

// Enquadramento errado é o risco real deste protocolo, porque o quadro binário
// não tem terminador. Este teste fixa o comportamento desejado: falhar, e não
// produzir uma posição plausível e falsa.
func TestMisframedBinaryFailsInsteadOfInventingPosition(t *testing.T) {
	frame := mustHex(t, realBinarySample)
	p := New(0)

	misframed := 0
	for offset := 1; offset <= 8; offset++ {
		shifted := append([]byte{markerBinary}, frame[offset:]...)
		msgs, err := p.Parse(shifted)
		if err != nil {
			misframed++
			continue
		}
		// Se por acaso decodificar, ao menos a coordenada precisa ser válida.
		if msgs[0].HasLocation && !protocols.ValidCoordinates(msgs[0].Latitude, msgs[0].Longitude) {
			t.Fatalf("offset %d produziu coordenada inválida sem erro", offset)
		}
	}
	if misframed == 0 {
		t.Fatal("nenhum deslocamento foi detectado: a validação não está protegendo nada")
	}
}

func TestDecodeBatteryTable(t *testing.T) {
	tests := []struct {
		value byte
		want  *int
	}{
		{0x00, nil},
		{0x01, protocols.Ptr(0)},
		{0x02, protocols.Ptr(10)},
		{0x03, protocols.Ptr(20)},
		{0x04, protocols.Ptr(60)},
		{0x06, protocols.Ptr(100)},
		{0x50, protocols.Ptr(80)},
		{0xF3, protocols.Ptr(3)},
		{0xFF, nil},
	}

	for _, tc := range tests {
		got := decodeBattery(tc.value)
		switch {
		case tc.want == nil && got != nil:
			t.Fatalf("0x%02X: esperava indefinido, recebi %d", tc.value, *got)
		case tc.want != nil && got == nil:
			t.Fatalf("0x%02X: esperava %d, recebi indefinido", tc.value, *tc.want)
		case tc.want != nil && *got != *tc.want:
			t.Fatalf("0x%02X: esperava %d, recebi %d", tc.value, *tc.want, *got)
		}
	}
}

// ---------------------------------------------------------------------------
// Palavra de estado
// ---------------------------------------------------------------------------

func TestStatusWordAlarms(t *testing.T) {
	tests := []struct {
		name      string
		status    uint32
		wantAlarm string
		wantACC   bool
	}{
		{name: "tudo normal, ignição ligada", status: 0xFFFFFFFF, wantAlarm: "", wantACC: true},
		{name: "ignição desligada", status: 0xFFFFFBFF, wantAlarm: "", wantACC: false},
		{name: "vibração", status: 0xFFFFFFFE, wantAlarm: "VIBRATION", wantACC: true},
		{name: "pânico", status: 0xFFFFFFFD, wantAlarm: "SOS", wantACC: true},
		{name: "excesso de velocidade", status: 0xFFFFFFFB, wantAlarm: "OVERSPEED", wantACC: true},
		{name: "corte de energia", status: 0xFFF7FFFF, wantAlarm: "POWER_LOSS", wantACC: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg := protocols.TrackerMessage{Attributes: map[string]any{}}
			applyStatus(&msg, tc.status)

			if msg.Alarm != tc.wantAlarm {
				t.Fatalf("alarme: esperava %q, recebi %q", tc.wantAlarm, msg.Alarm)
			}
			if msg.ACC == nil || *msg.ACC != tc.wantACC {
				t.Fatalf("ignição: esperava %v, recebi %v", tc.wantACC, msg.ACC)
			}
			if tc.wantAlarm != "" && msg.Kind != protocols.KindAlarm {
				t.Fatalf("mensagem com alarme deveria ser KindAlarm, veio %q", msg.Kind)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Quadro de texto
// ---------------------------------------------------------------------------

const (
	textPosition  = "*HQ,869247061234567,V1,134530,A,2332.5000,S,04638.0000,W,000.00,000,170926,FFFFFBFF#"
	textHeartbeat = "*HQ,869247061234567,HTBT,87#"
	textResponse  = "*HQ,869247061234567,V4,S20,134530#"
)

func TestDecodeTextPosition(t *testing.T) {
	msg := parseOne(t, newProtocol(), []byte(textPosition))

	if msg.Kind != protocols.KindPosition {
		t.Fatalf("esperava posição, recebi %q", msg.Kind)
	}
	if msg.IMEI != "869247061234567" {
		t.Fatalf("identificador: %q", msg.IMEI)
	}
	if !msg.GPSValid {
		t.Fatal("esperava fix válido")
	}

	wantLat := -(23 + 32.5/60)
	if math.Abs(msg.Latitude-wantLat) > 1e-9 {
		t.Fatalf("latitude: esperava %f, recebi %f", wantLat, msg.Latitude)
	}

	wantTime := time.Date(2026, 9, 17, 13, 45, 30, 0, time.UTC)
	if !msg.Timestamp.Equal(wantTime) {
		t.Fatalf("timestamp: esperava %s, recebi %s", wantTime, msg.Timestamp)
	}

	// A palavra de estado agora é decodificada com os mesmos bits do binário.
	if msg.ACC == nil || *msg.ACC {
		t.Fatalf("ignição: esperava desligada, recebi %v", msg.ACC)
	}
	if msg.Attributes["statusWord"] != "FFFFFBFF" {
		t.Fatalf("palavra de estado: %v", msg.Attributes["statusWord"])
	}
}

func TestDecodeTextSpeedInKnots(t *testing.T) {
	line := strings.Replace(textPosition, ",000.00,000,", ",054.00,270,", 1)
	msg := parseOne(t, newProtocol(), []byte(line))

	if math.Abs(msg.SpeedKmh-100.008) > 1e-9 {
		t.Fatalf("54 nós deveriam virar 100.008 km/h, recebi %v", msg.SpeedKmh)
	}
	if msg.Attributes["speedUnit"] != "knots" {
		t.Fatal("a unidade de origem precisa ficar registrada")
	}
	if msg.Heading != 270 {
		t.Fatalf("rumo: %v", msg.Heading)
	}
}

func TestDecodeTextHeartbeatAndResponse(t *testing.T) {
	heartbeat := parseOne(t, newProtocol(), []byte(textHeartbeat))
	if heartbeat.Kind != protocols.KindHeartbeat {
		t.Fatalf("esperava heartbeat, recebi %q", heartbeat.Kind)
	}
	if heartbeat.BatteryPercent == nil || *heartbeat.BatteryPercent != 87 {
		t.Fatalf("bateria: %v", heartbeat.BatteryPercent)
	}
	if heartbeat.HasLocation {
		t.Fatal("heartbeat não traz coordenada")
	}

	response := parseOne(t, newProtocol(), []byte(textResponse))
	if response.Kind != protocols.KindCommandAck {
		t.Fatalf("esperava command_ack, recebi %q", response.Kind)
	}
	if !strings.Contains(response.Response, "S20") {
		t.Fatalf("resposta: %q", response.Response)
	}
	// O protocolo de texto não carrega chave: o casamento é por ordem.
	if response.CorrelationKey != 0 {
		t.Fatalf("não deveria haver chave de correlação, veio %d", response.CorrelationKey)
	}
}

func TestDecodeTextRejectsBadIdentifier(t *testing.T) {
	if _, err := newProtocol().Parse([]byte("*HQ,123,V1,134530,A,2332.5000,S,04638.0000,W,0,0,170926,FF#")); err == nil {
		t.Fatal("identificador curto deveria ser recusado")
	}
}

func TestDecodeTextUnknownTypeIsCaptured(t *testing.T) {
	msg := parseOne(t, newProtocol(), []byte("*HQ,869247061234567,ZZZ,qualquer,coisa#"))

	if msg.Kind != protocols.KindOther {
		t.Fatalf("tipo desconhecido não pode virar %q", msg.Kind)
	}
	if msg.HasLocation {
		t.Fatal("tipo desconhecido não pode produzir coordenada")
	}
	if msg.Attributes["unrecognized"] != true {
		t.Fatal("deveria ficar marcado para análise")
	}
}

// ---------------------------------------------------------------------------
// Detecção e enquadramento
// ---------------------------------------------------------------------------

func TestDetect(t *testing.T) {
	p := newProtocol()

	if !p.Detect(mustHex(t, realBinarySample)) {
		t.Fatal("deveria reconhecer o quadro binário")
	}
	if !p.Detect([]byte(textPosition)) {
		t.Fatal("deveria reconhecer o quadro de texto")
	}
	if p.Detect([]byte{0x78, 0x78, 0x0D}) {
		t.Fatal("não deveria reconhecer GT06")
	}
	if p.Detect([]byte("imei:869247061234567,tracker,;")) {
		t.Fatal("não deveria reconhecer a família GPS103")
	}
	if p.Detect([]byte("*ZZ,nao-e-imei,V1,#")) {
		t.Fatal("não deveria reconhecer texto sem identificador válido")
	}
	if p.Detect(nil) {
		t.Fatal("não deveria decidir sem bytes")
	}
}

func TestNextFrameText(t *testing.T) {
	p := newProtocol()
	stream := []byte(textHeartbeat + textPosition)

	first, consumed, err := p.NextFrame(stream)
	if err != nil {
		t.Fatalf("primeiro quadro: %v", err)
	}
	if string(first) != textHeartbeat {
		t.Fatalf("primeiro quadro: %q", first)
	}

	second, _, err := p.NextFrame(stream[consumed:])
	if err != nil {
		t.Fatalf("segundo quadro: %v", err)
	}
	if string(second) != textPosition {
		t.Fatalf("segundo quadro: %q", second)
	}
}

func TestNextFrameTextIncomplete(t *testing.T) {
	p := newProtocol()
	partial := []byte(textPosition[:20])

	if _, consumed, err := p.NextFrame(partial); !errors.Is(err, protocols.ErrIncompleteFrame) || consumed != 0 {
		t.Fatalf("esperava ErrIncompleteFrame sem consumo, recebi %d / %v", consumed, err)
	}
}

func TestNextFrameBinaryWithFixedLength(t *testing.T) {
	// Com o tamanho fixado, dois quadros grudados são separados corretamente.
	p := New(47)
	frame := mustHex(t, realBinarySample)
	stream := append(append([]byte{}, frame...), frame...)

	first, consumed, err := p.NextFrame(stream)
	if err != nil {
		t.Fatalf("primeiro quadro: %v", err)
	}
	if consumed != 47 || len(first) != 47 {
		t.Fatalf("consumo inesperado: %d", consumed)
	}

	second, consumed, err := p.NextFrame(stream[consumed:])
	if err != nil || consumed != 47 || len(second) != 47 {
		t.Fatalf("segundo quadro: %d / %v", consumed, err)
	}
}

func TestNextFrameBinaryWaitsForFixedLength(t *testing.T) {
	p := New(47)
	frame := mustHex(t, realBinarySample)

	for cut := 1; cut < 47; cut++ {
		_, consumed, err := p.NextFrame(frame[:cut])
		if cut < minBinaryPayload && errors.Is(err, protocols.ErrIncompleteFrame) {
			continue
		}
		if !errors.Is(err, protocols.ErrIncompleteFrame) || consumed != 0 {
			t.Fatalf("corte em %d: esperava aguardar, recebi %d / %v", cut, consumed, err)
		}
	}
}

func TestNextFrameBinaryAutoTakesBuffer(t *testing.T) {
	// Sem tamanho configurado, a heurística é um quadro por leitura.
	p := New(0)
	frame := mustHex(t, realBinarySample)

	got, consumed, err := p.NextFrame(frame)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if consumed != len(frame) || len(got) != len(frame) {
		t.Fatalf("consumo inesperado: %d", consumed)
	}

	// Abaixo do mínimo decodificável, aguarda mais bytes.
	if _, _, err := p.NextFrame(frame[:10]); !errors.Is(err, protocols.ErrIncompleteFrame) {
		t.Fatalf("esperava ErrIncompleteFrame, recebi %v", err)
	}
}

func TestNextFrameSkipsGarbage(t *testing.T) {
	p := newProtocol()
	buf := append([]byte{0x00, 0x11, 0x22}, []byte(textHeartbeat)...)

	_, consumed, err := p.NextFrame(buf)
	if err == nil {
		t.Fatal("o lixo antes do marcador precisa ser reportado")
	}
	if consumed != 3 {
		t.Fatalf("deveria consumir 3 bytes de lixo, consumiu %d", consumed)
	}

	frame, _, err := p.NextFrame(buf[consumed:])
	if err != nil || string(frame) != textHeartbeat {
		t.Fatalf("quadro após o lixo: %q / %v", frame, err)
	}
}

// ---------------------------------------------------------------------------
// Comandos
// ---------------------------------------------------------------------------

func withFrozenClock(t *testing.T) {
	t.Helper()
	original := clock
	clock = func() time.Time { return time.Date(2026, 9, 20, 13, 45, 30, 0, time.UTC) }
	t.Cleanup(func() { clock = original })
}

func TestEncodeCommands(t *testing.T) {
	withFrozenClock(t)
	p := newProtocol()

	tests := []struct {
		name string
		cmd  protocols.Command
		want string
	}{
		{
			name: "corte de motor",
			cmd:  protocols.Command{Type: protocols.CommandEngineCut, UniqueID: "869247061234567"},
			want: "*HQ,869247061234567,S20,134530,1,1#",
		},
		{
			name: "liberação",
			cmd:  protocols.Command{Type: protocols.CommandEngineResume, UniqueID: "869247061234567"},
			want: "*HQ,869247061234567,S20,134530,1,0#",
		},
		{
			name: "intervalo",
			cmd: protocols.Command{
				Type: protocols.CommandSetInterval, UniqueID: "869247061234567",
				Params: map[string]string{"seconds": "60"},
			},
			want: "*HQ,869247061234567,S71,134530,22,60#",
		},
		{
			name: "override do dispositivo prevalece",
			cmd: protocols.Command{
				Type: protocols.CommandEngineCut, UniqueID: "869247061234567",
				Raw: "*HQ,869247061234567,S20,000000,1,1#",
			},
			want: "*HQ,869247061234567,S20,000000,1,1#",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := p.EncodeCommand(tc.cmd)
			if err != nil {
				t.Fatalf("EncodeCommand falhou: %v", err)
			}
			if string(got) != tc.want {
				t.Fatalf("esperava %q, recebi %q", tc.want, got)
			}
		})
	}
}

func TestEncodeCommandRequiresIdentifier(t *testing.T) {
	if _, err := newProtocol().EncodeCommand(protocols.Command{
		Type: protocols.CommandEngineCut,
	}); err == nil {
		t.Fatal("comando sem identificador deveria ser recusado")
	}
}

func TestEncodeUnconfirmedCommandsAreRefused(t *testing.T) {
	p := newProtocol()

	// Posição, status, reboot e servidor não têm texto confirmado aqui.
	for _, cmdType := range []protocols.CommandType{
		protocols.CommandRequestPosition, protocols.CommandRequestStatus,
		protocols.CommandReboot, protocols.CommandSetServer,
	} {
		_, err := p.EncodeCommand(protocols.Command{
			Type: cmdType, UniqueID: "869247061234567",
			Params: map[string]string{"host": "x.com", "port": "5000"},
		})
		if !errors.Is(err, protocols.ErrUnsupportedCommand) {
			t.Fatalf("%s: esperava ErrUnsupportedCommand, recebi %v", cmdType, err)
		}
	}
}

func TestEncodeCommandRejectsInjection(t *testing.T) {
	p := newProtocol()

	bad := []protocols.Command{
		{Type: protocols.CommandEngineCut, UniqueID: "8692470612345;S20"},
		{Type: protocols.CommandEngineCut, UniqueID: "869247061234567", Password: "12;RESET"},
		{Type: protocols.CommandEngineCut, UniqueID: "869247061234567", Raw: "*HQ\r\n,S20#"},
		{Type: protocols.CommandSetInterval, UniqueID: "869247061234567",
			Params: map[string]string{"seconds": "0"}},
		{Type: protocols.CommandCustom, UniqueID: "869247061234567"},
	}

	for i, cmd := range bad {
		if _, err := p.EncodeCommand(cmd); err == nil {
			t.Fatalf("caso %d: esperava recusa", i)
		}
	}
}

func TestDescribeDeclaresAssumed(t *testing.T) {
	descriptor := newProtocol().Describe()

	if descriptor.Confidence != protocols.Assumed {
		t.Fatalf("o H02 precisa se declarar ASSUMED, veio %q", descriptor.Confidence)
	}
	if !descriptor.Supports(protocols.CommandEngineCut) {
		t.Fatal("o descritor deveria listar o corte de motor")
	}
	if !strings.Contains(descriptor.Notes, "identificador curto") {
		t.Fatal("as observações precisam avisar sobre o identificador curto")
	}
}
