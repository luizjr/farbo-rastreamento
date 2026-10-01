package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/auth"
	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/config"
	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/melhorenvio"
	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/telemetry"
)

const webhookSecret = "secret-do-aplicativo-de-teste"

func webhookServer(t *testing.T, configured bool) http.Handler {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.Config{
		HTTP: config.HTTP{RateLimitRPS: 1000, RateLimitBurst: 1000},
		Auth: config.Auth{JWTSecret: rbacSecret, AccessTokenTTL: time.Hour},
	}
	deps := Deps{Config: cfg, Log: log, Metrics: telemetry.NewMetrics(), Auth: auth.NewService(nil, cfg.Auth, nil, log)}
	if configured {
		cfg.Shipping.ClientSecret = webhookSecret
		deps.Carrier = melhorenvio.New(melhorenvio.Config{BaseURL: "https://sandbox.melhorenvio.com.br", ClientSecret: webhookSecret}, nil)
	}
	return NewServer(deps).Handler()
}

func postWebhook(t *testing.T, handler http.Handler, body, signature string) (int, map[string]string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/shipping/melhorenvio/webhook", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if signature != "" {
		req.Header.Set("X-ME-Signature", signature)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	out := map[string]string{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

// sign assina como o Melhor Envios: HMAC-SHA256 do corpo com o secret, em base64.
func sign(body string) string {
	mac := hmac.New(sha256.New, []byte(webhookSecret))
	mac.Write([]byte(body))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func TestShippingWebhookAcceptsRegistrationTest(t *testing.T) {
	handler := webhookServer(t, true)
	// O teste de cadastro vem assinado, mas sem etiqueta: precisa de 200,
	// senão o Melhor Envios recusa o cadastro (E-WBH-0002).
	for _, body := range []string{`{}`, `{"event":"test"}`, `{"event":"order.created","data":{}}`, ``, `ping`} {
		status, out := postWebhook(t, handler, body, sign(body))
		if status != http.StatusOK || out["status"] != "ignorado" {
			t.Errorf("teste de cadastro %q: esperava 200 ignorado, veio %d %v", body, status, out)
		}
	}
}

func TestShippingWebhookRejectsUnsignedOrForged(t *testing.T) {
	handler := webhookServer(t, true)
	body := `{"event":"order.delivered","data":{"id":"abc","status":"delivered"}}`
	if status, _ := postWebhook(t, handler, body, ""); status != http.StatusUnauthorized {
		t.Errorf("sem assinatura: esperava 401, veio %d", status)
	}
	if status, _ := postWebhook(t, handler, body, sign(`{"outro":"corpo"}`)); status != http.StatusUnauthorized {
		t.Errorf("assinatura de outro corpo: esperava 401, veio %d", status)
	}
	// Teste de cadastro forjado (sem a assinatura certa) também é recusado.
	if status, _ := postWebhook(t, handler, `{}`, "assinatura-falsa"); status != http.StatusUnauthorized {
		t.Errorf("teste de cadastro forjado: esperava 401, veio %d", status)
	}
}

func TestShippingWebhookWithoutIntegration(t *testing.T) {
	handler := webhookServer(t, false)
	if status, out := postWebhook(t, handler, `{}`, sign(`{}`)); status != http.StatusServiceUnavailable || out["error"] == "" {
		t.Errorf("sem integração configurada: esperava 503, veio %d %v", status, out)
	}
}
