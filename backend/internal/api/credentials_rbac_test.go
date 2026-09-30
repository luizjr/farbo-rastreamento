package api

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/auth"
	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/config"
	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/telemetry"
)

// A trava de perfil vem antes do handler: as rotas administrativas de
// rastreador respondem 403 para quem não é admin sem tocar no banco. Por
// isso este teste roda sem Postgres (o de ponta a ponta, com banco, está em
// credentials_integration_test.go).

var rbacSecret = []byte("segredo-de-teste-rbac-0123456789abcdef")

func rbacServer(t *testing.T) http.Handler {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.Config{
		HTTP: config.HTTP{RateLimitRPS: 1000, RateLimitBurst: 1000},
		Auth: config.Auth{JWTSecret: rbacSecret, AccessTokenTTL: time.Hour},
	}
	return NewServer(Deps{
		Config: cfg, Log: log, Metrics: telemetry.NewMetrics(),
		Auth: auth.NewService(nil, cfg.Auth, nil, log),
	}).Handler()
}

// rbacToken assina um access token como o auth.Service faria.
func rbacToken(t *testing.T, role string) string {
	t.Helper()
	claims := auth.Claims{
		Role: role, Email: role + "@teste.local",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: uuid.NewString(), Issuer: "tracker-platform",
			IssuedAt: jwt.NewNumericDate(time.Now()), ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(rbacSecret)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func TestDeviceAdminRoutesForbiddenForOtherRoles(t *testing.T) {
	handler := rbacServer(t)
	device := "/api/devices/" + uuid.NewString()

	routes := []struct{ method, path, body string }{
		{http.MethodGet, device + "/provisioning", ""},
		{http.MethodPost, "/api/devices", `{"imei":"869247061230099"}`},
		{http.MethodPatch, device, `{"imei":"869247061230099","commandPassword":"Abc123"}`},
		{http.MethodDelete, device, ""},
	}
	for _, role := range []string{auth.RoleOperator, auth.RoleViewer, auth.RoleCustomer} {
		token := rbacToken(t, role)
		for _, route := range routes {
			req := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Errorf("%s %s como %s: esperava 403, veio %d (%s)",
					route.method, route.path, role, rec.Code, rec.Body)
			}
		}
	}

	// O cliente não lista nem abre rastreador pela rota da central.
	token := rbacToken(t, auth.RoleCustomer)
	for _, path := range []string{"/api/devices", device, device + "/commands", device + "/status"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("GET %s como cliente: esperava 403, veio %d", path, rec.Code)
		}
	}
}
