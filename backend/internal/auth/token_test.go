package auth

import (
	"net/http/httptest"
	"testing"
)

func TestBearerTokenOnlyTakesQueryOnWebSocket(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/vehicles?token=da-url", nil)
	if got := bearerToken(r); got != "" {
		t.Errorf("rota comum aceitou token da URL: %q", got)
	}

	r.Header.Set("Authorization", "Bearer do-cabecalho")
	if got := bearerToken(r); got != "do-cabecalho" {
		t.Errorf("cabeçalho devia valer, veio %q", got)
	}

	ws := httptest.NewRequest("GET", "/ws?token=da-url", nil)
	ws.Header.Set("Connection", "Upgrade")
	ws.Header.Set("Upgrade", "websocket")
	if got := bearerToken(ws); got != "da-url" {
		t.Errorf("WebSocket devia aceitar o token da URL, veio %q", got)
	}
}
