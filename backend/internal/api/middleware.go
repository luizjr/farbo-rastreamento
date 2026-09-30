package api

import (
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"golang.org/x/time/rate"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/telemetry"
)

// metricsMiddleware alimenta os contadores HTTP do Prometheus.
func metricsMiddleware(m *telemetry.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			recorder := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(recorder, r)

			// O padrão da rota (e não o caminho) evita explodir a cardinalidade
			// das métricas com um rótulo por UUID.
			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = "unmatched"
			}
			m.HTTPRequests.WithLabelValues(r.Method, route, strconv.Itoa(recorder.Status())).Inc()
			m.HTTPDuration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
		})
	}
}

// loggingMiddleware registra cada requisição em JSON estruturado.
func loggingMiddleware(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			recorder := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(recorder, r)

			level := slog.LevelInfo
			if recorder.Status() >= 500 {
				level = slog.LevelError
			} else if recorder.Status() >= 400 {
				level = slog.LevelWarn
			}

			// Nada de corpo, token ou credencial no log (§28).
			log.Log(r.Context(), level, "http",
				"method", r.Method,
				"path", r.URL.Path,
				"status", recorder.Status(),
				"bytes", recorder.BytesWritten(),
				"duration_ms", time.Since(start).Milliseconds(),
				"ip", clientIP(r),
				"request_id", middleware.GetReqID(r.Context()),
			)
		})
	}
}

// rateLimiter aplica um limite por IP (§27).
type rateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rps      rate.Limit
	burst    int
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newRateLimiter(rps float64, burst int) *rateLimiter {
	rl := &rateLimiter{
		visitors: make(map[string]*visitor),
		rps:      rate.Limit(rps),
		burst:    burst,
	}
	go rl.cleanup()
	return rl
}

func (rl *rateLimiter) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		for ip, v := range rl.visitors {
			if time.Since(v.lastSeen) > 10*time.Minute {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, ok := rl.visitors[ip]
	if !ok {
		v = &visitor{limiter: rate.NewLimiter(rl.rps, rl.burst)}
		rl.visitors[ip] = v
	}
	v.lastSeen = time.Now()
	return v.limiter.Allow()
}

func (rl *rateLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.allow(clientIP(r)) {
			w.Header().Set("Retry-After", "1")
			writeError(w, http.StatusTooManyRequests, "muitas requisições; tente novamente em instantes")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// trustedProxies são as redes de quem o X-Forwarded-For é aceito
// (HTTP.TrustedProxies / TRUSTED_PROXIES); definido na montagem do servidor.
var trustedProxies []netip.Prefix

func isTrustedProxy(ip netip.Addr) bool {
	ip = ip.Unmap()
	for _, prefix := range trustedProxies {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}

// clientIP é o IP de origem. Os cabeçalhos de proxy (X-Forwarded-For,
// X-Real-IP) só valem quando a conexão vem de um proxy confiável; aí o
// X-Forwarded-For é lido da direita para a esquerda, pulando os proxies —
// o primeiro endereço que sobra é o cliente (o que ele mesmo escreveu à
// esquerda é ignorado).
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer, err := netip.ParseAddr(host)
	if err != nil || !isTrustedProxy(peer) {
		return host
	}
	if forwarded := r.Header.Values("X-Forwarded-For"); len(forwarded) > 0 {
		hops := strings.Split(strings.Join(forwarded, ","), ",")
		for i := len(hops) - 1; i >= 0; i-- {
			hop, err := netip.ParseAddr(trimSpace(hops[i]))
			if err != nil {
				// Lixo no cabeçalho: não dá para confiar no que vem antes.
				break
			}
			if !isTrustedProxy(hop) {
				return hop.Unmap().String()
			}
		}
	}
	if realIP, err := netip.ParseAddr(trimSpace(r.Header.Get("X-Real-IP"))); err == nil {
		return realIP.Unmap().String()
	}
	return host
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
