// Command loadgen é o teste de carga: simula muitos rastreadores GT06 (o
// protocolo dos J16) mandando posição no intervalo configurado e, se pedido,
// painéis abertos no WebSocket recebendo tudo em tempo real.
//
// Os IMEIs são prefixo + número sequencial (000001, 000002, ...) e precisam
// estar cadastrados no servidor, senão a conexão é recusada.
//
//	go run ./cmd/loadgen -host 127.0.0.1 -port 5000 -devices 5000 -interval 10s -duration 10m
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gorilla/websocket"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/protocols/gt06"
)

type counters struct {
	connected    atomic.Int64
	loggedIn     atomic.Int64
	sent         atomic.Int64
	sendErrors   atomic.Int64
	dialErrors   atomic.Int64
	loginErrors  atomic.Int64
	reconnects   atomic.Int64
	wsMessages   atomic.Int64
	wsClients    atomic.Int64
	loginLatency latencies
}

// latencies guarda amostras para percentis.
type latencies struct {
	mu      sync.Mutex
	samples []time.Duration
}

func (l *latencies) add(d time.Duration) {
	l.mu.Lock()
	l.samples = append(l.samples, d)
	l.mu.Unlock()
}

func (l *latencies) percentile(p float64) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.samples) == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), l.samples...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return sorted[int(math.Min(float64(len(sorted)-1), p*float64(len(sorted))))]
}

func main() {
	host := flag.String("host", "127.0.0.1", "servidor TCP dos rastreadores")
	port := flag.Int("port", 5000, "porta TCP dos rastreadores")
	devices := flag.Int("devices", 5000, "quantos rastreadores simular")
	prefix := flag.String("imei-prefix", "869", "prefixo dos IMEIs (completados até 15 dígitos)")
	interval := flag.Duration("interval", 10*time.Second, "intervalo entre posições de cada rastreador")
	heartbeat := flag.Duration("heartbeat", 3*time.Minute, "intervalo do heartbeat")
	ramp := flag.Duration("ramp", time.Minute, "tempo para conectar todos (0 = todos de uma vez)")
	duration := flag.Duration("duration", 10*time.Minute, "duração total, contando a rampa")
	apiURL := flag.String("api", "", "URL do painel (ex.: http://127.0.0.1:8080) para os clientes WebSocket")
	wsClients := flag.Int("ws", 0, "quantos painéis abertos simular no WebSocket")
	email := flag.String("email", "", "login de quem abre o painel")
	password := flag.String("password", "", "senha de quem abre o painel")
	origin := flag.String("origin", "http://localhost:5173", "Origin aceito pelo WebSocket")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	ctx, stop := context.WithTimeout(ctx, *duration)
	defer stop()

	c := &counters{}
	start := time.Now()
	var wg sync.WaitGroup

	if *wsClients > 0 {
		token, err := login(*apiURL, *email, *password)
		if err != nil {
			fmt.Fprintln(os.Stderr, "login do painel:", err)
			os.Exit(1)
		}
		for i := 0; i < *wsClients; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				runPanel(ctx, *apiURL, token, *origin, c)
			}()
		}
	}

	addr := net.JoinHostPort(*host, fmt.Sprint(*port))
	for i := 1; i <= *devices; i++ {
		imei := *prefix + fmt.Sprintf("%0*d", 15-len(*prefix), i)
		delay := time.Duration(0)
		if *ramp > 0 {
			delay = time.Duration(float64(*ramp) * float64(i-1) / float64(*devices))
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
			runDevice(ctx, addr, imei, *interval, *heartbeat, c)
		}()
	}

	// Um resumo por intervalo, em JSON por linha (fácil de juntar depois).
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	var lastSent, lastWS int64
	lastAt := time.Now()
	report := func(final bool) {
		now := time.Now()
		sent, ws := c.sent.Load(), c.wsMessages.Load()
		secs := now.Sub(lastAt).Seconds()
		line := map[string]any{
			"t": math.Round(now.Sub(start).Seconds()), "connected": c.connected.Load(), "loggedIn": c.loggedIn.Load(),
			"sentPerSec": math.Round(float64(sent-lastSent) / secs), "sentTotal": sent,
			"sendErrors": c.sendErrors.Load(), "dialErrors": c.dialErrors.Load(), "loginErrors": c.loginErrors.Load(),
			"reconnects": c.reconnects.Load(), "wsClients": c.wsClients.Load(),
			"wsMsgPerSec": math.Round(float64(ws-lastWS) / secs),
			"loginP50ms":  c.loginLatency.percentile(0.5).Milliseconds(),
			"loginP99ms":  c.loginLatency.percentile(0.99).Milliseconds(),
		}
		if final {
			line["final"] = true
		}
		out, _ := json.Marshal(line)
		fmt.Println(string(out))
		lastSent, lastWS, lastAt = sent, ws, now
	}
	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			report(true)
			return
		case <-ticker.C:
			report(false)
		}
	}
}

// runDevice mantém um rastreador conectado, reconectando se cair.
func runDevice(ctx context.Context, addr, imei string, interval, heartbeat time.Duration, c *counters) {
	backoff := time.Second
	first := true
	for ctx.Err() == nil {
		if !first {
			c.reconnects.Add(1)
		}
		first = false
		err := session(ctx, addr, imei, interval, heartbeat, c)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff + time.Duration(rand.Int64N(int64(time.Second)))):
			}
			backoff = min(backoff*2, 30*time.Second)
		}
	}
}

func session(ctx context.Context, addr, imei string, interval, heartbeat time.Duration, c *counters) error {
	conn, err := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp", addr)
	if err != nil {
		c.dialErrors.Add(1)
		return err
	}
	defer conn.Close()
	c.connected.Add(1)
	defer c.connected.Add(-1)

	var serial uint16 = 1
	next := func() uint16 { serial++; return serial }
	// Prazo renovado a cada envio: um prazo antigo derrubaria um heartbeat
	// que saísse logo depois dele.
	write := func(frame []byte) error {
		_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		_, err := conn.Write(frame)
		return err
	}

	frame, err := gt06.BuildLoginFrame(imei, next())
	if err != nil {
		return err
	}
	began := time.Now()
	if err := write(frame); err != nil {
		c.sendErrors.Add(1)
		return err
	}
	// O servidor confirma o login; sem confirmação, o IMEI foi recusado.
	_ = conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	reader := bufio.NewReader(conn)
	if _, err := reader.Peek(10); err != nil {
		c.loginErrors.Add(1)
		return fmt.Errorf("login sem resposta: %w", err)
	}
	c.loginLatency.add(time.Since(began))
	c.loggedIn.Add(1)
	defer c.loggedIn.Add(-1)
	_ = conn.SetReadDeadline(time.Time{})

	// Esvazia o que o servidor mandar (confirmações, comandos).
	go func() { _, _ = io.Copy(io.Discard, reader) }()

	// Cada rastreador começa num ponto aleatório da Grande São Paulo e anda.
	lat := -23.55 + (rand.Float64()-0.5)*0.4
	lon := -46.63 + (rand.Float64()-0.5)*0.4
	course := rand.Float64() * 360
	status := gt06.DeviceStatus{ACC: true, BatteryLevel: 6, GSMLevel: 4}

	// Fase aleatória: os envios se espalham pelo intervalo, como na vida real.
	positionTimer := time.NewTimer(time.Duration(rand.Int64N(int64(interval))))
	defer positionTimer.Stop()
	heartbeatTicker := time.NewTicker(heartbeat)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-heartbeatTicker.C:
			if err := write(gt06.BuildHeartbeatFrame(status, next())); err != nil {
				c.sendErrors.Add(1)
				return err
			}
		case <-positionTimer.C:
			speed := 20 + rand.Float64()*60
			course = math.Mod(course+(rand.Float64()-0.5)*30+360, 360)
			step := speed / 3600 * interval.Seconds() / 111
			lat += step * math.Cos(course*math.Pi/180)
			lon += step * math.Sin(course*math.Pi/180)
			fix := gt06.Fix{Time: time.Now(), Latitude: lat, Longitude: lon, SpeedKmh: speed,
				CourseDeg: course, Satellites: 9, Valid: true}
			if err := write(gt06.BuildPositionFrame(fix, status, next())); err != nil {
				c.sendErrors.Add(1)
				return err
			}
			c.sent.Add(1)
			positionTimer.Reset(interval)
		}
	}
}

func login(api, email, password string) (string, error) {
	body := strings.NewReader(fmt.Sprintf(`{"email":%q,"password":%q}`, email, password))
	resp, err := http.Post(strings.TrimRight(api, "/")+"/api/auth/login", "application/json", body)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || out.AccessToken == "" {
		return "", fmt.Errorf("login recusado (%d)", resp.StatusCode)
	}
	return out.AccessToken, nil
}

// runPanel é um painel aberto: recebe todas as posições pelo WebSocket.
func runPanel(ctx context.Context, api, token, origin string, c *counters) {
	u, _ := url.Parse(strings.TrimRight(api, "/"))
	u.Scheme = strings.Replace(u.Scheme, "http", "ws", 1)
	u.Path = "/ws"
	u.RawQuery = "token=" + url.QueryEscape(token)
	for ctx.Err() == nil {
		conn, _, err := websocket.DefaultDialer.DialContext(ctx, u.String(), http.Header{"Origin": {origin}})
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		c.wsClients.Add(1)
		go func() {
			<-ctx.Done()
			conn.Close()
		}()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
			c.wsMessages.Add(1)
		}
		c.wsClients.Add(-1)
		conn.Close()
	}
}
