// Comando tksim: simulador de rastreador para desenvolvimento (§32).
//
// Conecta no servidor TCP, faz login, envia posição e heartbeat, recebe
// comandos e responde ACK — inclusive acionando e liberando o relé virtual,
// que é o que permite testar o fluxo de corte de motor sem hardware.
//
// Uso:
//
//	go run ./cmd/tksim --imei 869247061234567 --host localhost --port 5000 --interval 10
//
// Teclas: o simulador também aceita comandos pela entrada padrão:
//
//	acc on | acc off | speed <km/h> | sos | move | stop | status | quit
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/protocols"
	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/protocols/gt06"
)

type options struct {
	imei      string
	host      string
	port      int
	interval  time.Duration
	heartbeat time.Duration
	lat       float64
	lng       float64
	speed     float64
	acc       bool
	moving    bool
	protocol  string
	// echo imita o firmware que devolve o comando recebido na resposta —
	// senha inclusa. Serve para conferir que o painel não a mostra.
	echo bool
}

func main() {
	opts := parseFlags()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	sim := newSimulator(opts)
	if err := sim.run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(os.Stderr, "simulador encerrado: %v\n", err)
		os.Exit(1)
	}
}

func parseFlags() options {
	var opts options
	flag.StringVar(&opts.imei, "imei", "869247061234567", "IMEI do rastreador simulado")
	flag.StringVar(&opts.host, "host", "localhost", "host do servidor TCP")
	flag.IntVar(&opts.port, "port", 5000, "porta do servidor TCP")
	flag.DurationVar(&opts.interval, "interval", 10*time.Second, "intervalo entre posições")
	flag.DurationVar(&opts.heartbeat, "heartbeat", 60*time.Second, "intervalo entre heartbeats")
	flag.Float64Var(&opts.lat, "lat", -23.5505, "latitude inicial")
	flag.Float64Var(&opts.lng, "lng", -46.6333, "longitude inicial")
	flag.Float64Var(&opts.speed, "speed", 0, "velocidade inicial em km/h")
	flag.BoolVar(&opts.acc, "acc", false, "começar com a ignição ligada")
	flag.BoolVar(&opts.moving, "moving", false, "deslocar o veículo a cada posição")
	flag.StringVar(&opts.protocol, "protocol", "gt06", "protocolo simulado (gt06)")
	flag.BoolVar(&opts.echo, "echo", false, "repetir o comando recebido na resposta (como alguns firmwares)")
	flag.Parse()

	if err := protocols.ValidateIMEI(opts.imei); err != nil {
		fmt.Fprintf(os.Stderr, "IMEI inválido: %v\n", err)
		os.Exit(2)
	}
	if opts.protocol != "gt06" {
		fmt.Fprintf(os.Stderr,
			"protocolo %q ainda não é simulado; use gt06 (o único com formato confirmado)\n",
			opts.protocol)
		os.Exit(2)
	}
	return opts
}

type simulator struct {
	opts options

	mu      sync.Mutex
	lat     float64
	lng     float64
	speed   float64
	course  float64
	acc     bool
	relayOn bool
	battery byte
	gsm     byte

	conn   net.Conn
	serial uint16
	writeM sync.Mutex

	alarms chan byte
}

func newSimulator(opts options) *simulator {
	return &simulator{
		opts:    opts,
		lat:     opts.lat,
		lng:     opts.lng,
		speed:   opts.speed,
		course:  90,
		acc:     opts.acc,
		battery: 5,
		gsm:     4,
		alarms:  make(chan byte, 8),
	}
}

func (s *simulator) run(ctx context.Context) error {
	addr := net.JoinHostPort(s.opts.host, strconv.Itoa(s.opts.port))

	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("conectando em %s: %w", addr, err)
	}
	s.conn = conn
	defer func() { _ = conn.Close() }()

	fmt.Printf("conectado em %s como IMEI %s\n", addr, s.opts.imei)

	login, err := gt06.BuildLoginFrame(s.opts.imei, s.nextSerial())
	if err != nil {
		return err
	}
	if err := s.write(login); err != nil {
		return fmt.Errorf("enviando login: %w", err)
	}
	fmt.Println("-> login enviado")

	go s.readLoop(ctx)
	go s.consoleLoop(ctx)

	positionTicker := time.NewTicker(s.opts.interval)
	heartbeatTicker := time.NewTicker(s.opts.heartbeat)
	defer positionTicker.Stop()
	defer heartbeatTicker.Stop()

	// Primeira posição logo após o login, para o painel já mostrar o veículo.
	time.Sleep(300 * time.Millisecond)
	s.sendPosition()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("encerrando simulador")
			return nil
		case <-positionTicker.C:
			s.sendPosition()
		case <-heartbeatTicker.C:
			s.sendHeartbeat()
		case code := <-s.alarms:
			s.sendAlarm(code)
		}
	}
}

// readLoop recebe os comandos do servidor e responde o ACK.
func (s *simulator) readLoop(ctx context.Context) {
	proto := gt06.New(false)
	reader := bufio.NewReader(s.conn)
	buf := make([]byte, 0, 1024)
	chunk := make([]byte, 512)

	for ctx.Err() == nil {
		n, err := reader.Read(chunk)
		if n > 0 {
			buf = append(buf, chunk[:n]...)
			buf = s.consume(proto, buf)
		}
		if err != nil {
			if ctx.Err() == nil {
				fmt.Fprintf(os.Stderr, "conexão encerrada pelo servidor: %v\n", err)
			}
			return
		}
	}
}

func (s *simulator) consume(proto *gt06.Protocol, buf []byte) []byte {
	for len(buf) > 0 {
		frame, consumed, err := proto.NextFrame(buf)
		if errors.Is(err, protocols.ErrIncompleteFrame) {
			return buf
		}
		if err != nil {
			if consumed <= 0 {
				return nil
			}
			buf = buf[consumed:]
			continue
		}
		buf = buf[consumed:]
		if frame != nil {
			s.handleServerFrame(frame)
		}
	}
	return buf
}

func (s *simulator) handleServerFrame(frame []byte) {
	serverFlag, text, err := gt06.DecodeServerCommand(frame)
	if err != nil {
		// ACKs do servidor (login, heartbeat) caem aqui e são esperados.
		return
	}
	fmt.Printf("<- comando recebido: %q (flag %d)\n", text, serverFlag)

	reply := s.applyCommand(text)
	if s.opts.echo {
		reply = text + " " + reply
	}
	time.Sleep(400 * time.Millisecond) // o aparelho real leva um instante

	if err := s.write(gt06.BuildCommandReply(serverFlag, reply, s.nextSerial())); err != nil {
		fmt.Fprintf(os.Stderr, "falha ao responder comando: %v\n", err)
		return
	}
	fmt.Printf("-> resposta enviada: %q\n", reply)

	// Depois de mexer no relé, o aparelho manda o estado novo.
	if strings.HasPrefix(text, "DYD") || strings.HasPrefix(text, "HFYD") ||
		strings.HasPrefix(text, "RELAY") {
		s.sendPosition()
	}
}

// applyCommand aplica o comando ao estado simulado e devolve a resposta.
func (s *simulator) applyCommand(text string) string {
	upper := strings.ToUpper(text)

	switch {
	case strings.HasPrefix(upper, "DYD"), strings.HasPrefix(upper, "RELAY,1"):
		s.mu.Lock()
		moving := s.speed > 20
		if !moving {
			s.relayOn = true
		}
		s.mu.Unlock()
		if moving {
			// O firmware real também se protege: não corta em movimento.
			return "DYD=Fail! Speed too high"
		}
		return "DYD=Success! Oil and Power OFF"

	case strings.HasPrefix(upper, "HFYD"), strings.HasPrefix(upper, "RELAY,0"):
		s.mu.Lock()
		s.relayOn = false
		s.mu.Unlock()
		return "HFYD=Success! Oil and Power ON"

	case strings.HasPrefix(upper, "WHERE"):
		s.mu.Lock()
		lat, lng := s.lat, s.lng
		s.mu.Unlock()
		defer s.sendPosition()
		return fmt.Sprintf("Lat:%.6f Lon:%.6f", lat, lng)

	case strings.HasPrefix(upper, "STATUS"):
		s.mu.Lock()
		defer s.mu.Unlock()
		return fmt.Sprintf("Battery:%d%% GSM:%d ACC:%s Relay:%s",
			int(s.battery)*100/6, s.gsm, onOff(s.acc), onOff(s.relayOn))

	case strings.HasPrefix(upper, "RESET"):
		return "RESET=Success!"

	case strings.HasPrefix(upper, "TIMER"), strings.HasPrefix(upper, "HBT"),
		strings.HasPrefix(upper, "SERVER"):
		return strings.SplitN(upper, ",", 2)[0] + "=Success!"

	default:
		return "Command not support"
	}
}

func onOff(v bool) string {
	if v {
		return "ON"
	}
	return "OFF"
}

// consoleLoop permite mudar o estado do veículo enquanto o simulador roda.
func (s *simulator) consoleLoop(ctx context.Context) {
	fmt.Println("comandos: acc on | acc off | speed <km/h> | move | stop | sos | status | quit")
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}
		line := strings.TrimSpace(strings.ToLower(scanner.Text()))
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		switch fields[0] {
		case "acc":
			if len(fields) < 2 {
				fmt.Println("uso: acc on|off")
				continue
			}
			s.mu.Lock()
			s.acc = fields[1] == "on"
			s.mu.Unlock()
			fmt.Printf("ignição %s\n", fields[1])
			s.sendPosition()

		case "speed":
			if len(fields) < 2 {
				fmt.Println("uso: speed <km/h>")
				continue
			}
			value, err := strconv.ParseFloat(fields[1], 64)
			if err != nil {
				fmt.Println("velocidade inválida")
				continue
			}
			s.mu.Lock()
			s.speed = value
			s.mu.Unlock()
			fmt.Printf("velocidade: %.1f km/h\n", value)
			s.sendPosition()

		case "move":
			s.opts.moving = true
			fmt.Println("veículo em deslocamento")

		case "stop":
			s.opts.moving = false
			s.mu.Lock()
			s.speed = 0
			s.mu.Unlock()
			fmt.Println("veículo parado")
			s.sendPosition()

		case "sos":
			s.alarms <- 0x01
		case "power":
			s.alarms <- 0x02

		case "status":
			s.mu.Lock()
			fmt.Printf("lat=%.6f lng=%.6f speed=%.1f acc=%s relay=%s\n",
				s.lat, s.lng, s.speed, onOff(s.acc), onOff(s.relayOn))
			s.mu.Unlock()

		case "quit", "exit":
			_ = s.conn.Close()
			return

		default:
			fmt.Println("comando desconhecido")
		}
	}
}

func (s *simulator) snapshot() (gt06.Fix, gt06.DeviceStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.opts.moving && s.speed > 0 {
		// Avança ao longo do rumo atual pelo tempo de um intervalo.
		meters := s.speed / 3.6 * s.opts.interval.Seconds()
		radians := s.course * math.Pi / 180
		s.lat += (meters * math.Cos(radians)) / 111132
		s.lng += (meters * math.Sin(radians)) / (111320 * math.Cos(s.lat*math.Pi/180))
		// Uma curva leve deixa o trajeto menos artificial no mapa.
		s.course = math.Mod(s.course+rand.Float64()*10-5+360, 360)
	}

	fix := gt06.Fix{
		Time:       time.Now().UTC(),
		Latitude:   s.lat,
		Longitude:  s.lng,
		SpeedKmh:   s.speed,
		CourseDeg:  s.course,
		Satellites: 9,
		Valid:      true,
	}
	status := gt06.DeviceStatus{
		ACC:          s.acc,
		RelayOn:      s.relayOn,
		Charging:     s.acc,
		BatteryLevel: s.battery,
		GSMLevel:     s.gsm,
	}
	return fix, status
}

func (s *simulator) sendPosition() {
	fix, status := s.snapshot()
	if err := s.write(gt06.BuildPositionFrame(fix, status, s.nextSerial())); err != nil {
		fmt.Fprintf(os.Stderr, "falha ao enviar posição: %v\n", err)
		return
	}
	fmt.Printf("-> posição %.6f,%.6f %.1f km/h acc=%s relay=%s\n",
		fix.Latitude, fix.Longitude, fix.SpeedKmh, onOff(status.ACC), onOff(status.RelayOn))
}

func (s *simulator) sendHeartbeat() {
	_, status := s.snapshot()
	if err := s.write(gt06.BuildHeartbeatFrame(status, s.nextSerial())); err != nil {
		fmt.Fprintf(os.Stderr, "falha ao enviar heartbeat: %v\n", err)
		return
	}
	fmt.Println("-> heartbeat")
}

func (s *simulator) sendAlarm(code byte) {
	fix, status := s.snapshot()
	if err := s.write(gt06.BuildAlarmFrame(fix, status, code, s.nextSerial())); err != nil {
		fmt.Fprintf(os.Stderr, "falha ao enviar alarme: %v\n", err)
		return
	}
	fmt.Printf("-> alarme 0x%02X\n", code)
}

func (s *simulator) write(payload []byte) error {
	s.writeM.Lock()
	defer s.writeM.Unlock()
	if err := s.conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	_, err := s.conn.Write(payload)
	return err
}

func (s *simulator) nextSerial() uint16 {
	s.writeM.Lock()
	defer s.writeM.Unlock()
	s.serial++
	if s.serial == 0 {
		s.serial = 1
	}
	return s.serial
}
