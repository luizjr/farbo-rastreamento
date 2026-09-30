package melhorenvio

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

type memStore struct {
	mu    sync.Mutex
	token *Token
	saves int
}

func (m *memStore) Load(context.Context) (*Token, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.token == nil {
		return nil, nil
	}
	copy := *m.token
	return &copy, nil
}
func (m *memStore) Save(_ context.Context, t *Token) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	copy := *t
	m.token, m.saves = &copy, m.saves+1
	return nil
}
func (m *memStore) Delete(context.Context) error { m.token = nil; return nil }

func newClient(t *testing.T, handler http.HandlerFunc, store TokenStore) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return New(Config{
		BaseURL: srv.URL, ClientID: "123", ClientSecret: "segredo", RedirectURL: "http://painel/api/cb",
		ContactEmail: "tec@farbo.test",
	}, store)
}

func TestAuthorizeURL(t *testing.T) {
	c := New(Config{BaseURL: SandboxURL, ClientID: "123", RedirectURL: "http://painel/api/cb"}, &memStore{})
	raw := c.AuthorizeURL("abc")
	if strings.Contains(raw, "+") {
		t.Errorf("escopos devem ir separados por %%20, não +: %s", raw)
	}
	u, _ := url.Parse(raw)
	q := u.Query()
	if u.Path != "/oauth/authorize" || q.Get("client_id") != "123" || q.Get("state") != "abc" ||
		q.Get("response_type") != "code" || q.Get("redirect_uri") != "http://painel/api/cb" {
		t.Fatalf("URL de autorização errada: %s", raw)
	}
	if !strings.Contains(q.Get("scope"), "shipping-checkout") || !strings.Contains(q.Get("scope"), " ") {
		t.Errorf("escopos: %q", q.Get("scope"))
	}
}

func TestExchangeCodeSavesToken(t *testing.T) {
	store := &memStore{}
	c := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if r.URL.Path != "/oauth/token" || body["grant_type"] != "authorization_code" || body["code"] != "xyz" ||
			body["client_secret"] != "segredo" || body["redirect_uri"] != "http://painel/api/cb" {
			t.Errorf("pedido de token errado: %s %v", r.URL.Path, body)
		}
		if !strings.Contains(r.Header.Get("User-Agent"), "tec@farbo.test") {
			t.Errorf("User-Agent sem e-mail de contato: %q", r.Header.Get("User-Agent"))
		}
		_, _ = w.Write([]byte(`{"token_type":"Bearer","expires_in":2592000,"access_token":"A1","refresh_token":"R1"}`))
	}, store)
	if err := c.ExchangeCode(context.Background(), "xyz"); err != nil {
		t.Fatal(err)
	}
	if store.token == nil || store.token.AccessToken != "A1" || store.token.RefreshToken != "R1" ||
		time.Until(store.token.ExpiresAt) < 29*24*time.Hour {
		t.Fatalf("token não guardado direito: %+v", store.token)
	}
}

func TestRefreshBeforeExpiry(t *testing.T) {
	store := &memStore{token: &Token{AccessToken: "velho", RefreshToken: "R1", ExpiresAt: time.Now().Add(time.Hour)}}
	var calls []string
	c := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.Path+" "+r.Header.Get("Authorization"))
		if r.URL.Path == "/oauth/token" {
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["grant_type"] != "refresh_token" || body["refresh_token"] != "R1" {
				t.Errorf("refresh errado: %v", body)
			}
			_, _ = w.Write([]byte(`{"expires_in":2592000,"access_token":"novo","refresh_token":"R2"}`))
			return
		}
		_, _ = w.Write([]byte(`{"firstname":"Pedro","email":"p@x"}`))
	}, store)
	acc, err := c.Account(context.Background())
	if err != nil || acc.FirstName != "Pedro" {
		t.Fatalf("conta: %v %v", acc, err)
	}
	if len(calls) != 2 || calls[1] != "/api/v2/me Bearer novo" {
		t.Fatalf("esperava renovar antes (falta < 1 dia) e usar o novo: %v", calls)
	}
	if store.token.RefreshToken != "R2" {
		t.Errorf("refresh token novo não guardado")
	}
}

func TestRetryOnUnauthorized(t *testing.T) {
	store := &memStore{token: &Token{AccessToken: "revogado", RefreshToken: "R1", ExpiresAt: time.Now().Add(20 * 24 * time.Hour)}}
	attempts := 0
	c := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/oauth/token":
			_, _ = w.Write([]byte(`{"expires_in":2592000,"access_token":"bom","refresh_token":"R2"}`))
		case r.Header.Get("Authorization") == "Bearer revogado":
			attempts++
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"Unauthenticated."}`))
		default:
			attempts++
			_, _ = w.Write([]byte(`{"firstname":"Ok"}`))
		}
	}, store)
	if _, err := c.Account(context.Background()); err != nil {
		t.Fatalf("depois do 401 devia renovar e repetir: %v", err)
	}
	if attempts != 2 {
		t.Errorf("tentativas = %d, quer 2", attempts)
	}
}

func TestNotConnected(t *testing.T) {
	c := newClient(t, func(w http.ResponseWriter, r *http.Request) { t.Error("não devia chamar a API") }, &memStore{})
	if _, err := c.Account(context.Background()); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("sem token: quer ErrNotConnected, veio %v", err)
	}
}

func TestRefreshRejectedMeansReconnect(t *testing.T) {
	store := &memStore{token: &Token{AccessToken: "x", RefreshToken: "vencido", ExpiresAt: time.Now()}}
	c := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","message":"The refresh token is invalid."}`))
	}, store)
	if _, err := c.Account(context.Background()); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("refresh recusado: quer ErrNotConnected, veio %v", err)
	}
}

func TestCalculateParsesAndSorts(t *testing.T) {
	store := &memStore{token: &Token{AccessToken: "A", RefreshToken: "R", ExpiresAt: time.Now().Add(20 * 24 * time.Hour)}}
	c := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["from"].(map[string]any)["postal_code"] != "01310100" {
			t.Errorf("CEP de origem não saiu só com dígitos: %v", body["from"])
		}
		_, _ = w.Write([]byte(`[
			{"id":2,"name":"SEDEX","price":"45.10","custom_price":"45.10","delivery_time":2,"custom_delivery_time":3,"company":{"name":"Correios"}},
			{"id":3,"name":".Package","error":"Serviço indisponível para o trecho.","company":{"name":"Jadlog"}},
			{"id":1,"name":"PAC","price":37.79,"delivery_time":8,"company":{"name":"Correios"}}]`))
	}, store)
	quotes, err := c.Calculate(context.Background(), "01310-100", "20040-020",
		Package{HeightCm: 5, WidthCm: 12, LengthCm: 16, WeightKg: 0.3, InsuranceCents: 15000}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(quotes) != 3 || quotes[0].ServiceID != 1 || quotes[0].PriceCents != 3779 || quotes[0].Name() != "Correios PAC" {
		t.Fatalf("cotação: %+v", quotes)
	}
	if quotes[1].PriceCents != 4510 || quotes[1].DeliveryDays != 3 {
		t.Errorf("preço/prazo personalizados: %+v", quotes[1])
	}
	if quotes[2].Error == "" {
		t.Errorf("serviço indisponível devia ir para o fim, com o erro: %+v", quotes[2])
	}
}

func TestErrorMessageShowsFirstFieldError(t *testing.T) {
	msg := errorMessage(422, []byte(`{"message":"The given data was invalid.","errors":{"to.document":["O documento é inválido."]}}`))
	if msg != "The given data was invalid. (to.document: O documento é inválido.)" {
		t.Errorf("mensagem: %q", msg)
	}
}

func TestVerifySignature(t *testing.T) {
	body := []byte(`{"event":"order.posted","data":{"id":"x"}}`)
	mac := hmac.New(sha256.New, []byte("segredo"))
	mac.Write(body)
	good := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if !VerifySignature("segredo", body, good) {
		t.Error("assinatura válida recusada")
	}
	if VerifySignature("segredo", append(body, ' '), good) || VerifySignature("outro", body, good) || VerifySignature("", body, good) {
		t.Error("assinatura inválida aceita")
	}
}

func TestTrackingCodePrefersCarrier(t *testing.T) {
	carrier, me := "BR123", "ME999"
	if (TrackingInfo{Tracking: &carrier, MelhorEnvioTracking: &me}).Code() != "BR123" {
		t.Error("devia preferir o código da transportadora")
	}
	empty := ""
	if (TrackingInfo{Tracking: &empty, MelhorEnvioTracking: &me}).Code() != "ME999" {
		t.Error("sem o da transportadora, usa o do Melhor Envios")
	}
}

func TestGenerateReadsLooseMessages(t *testing.T) {
	store := &memStore{token: &Token{AccessToken: "A", RefreshToken: "R", ExpiresAt: time.Now().Add(20 * 24 * time.Hour)}}
	reply := `{"abc":{"status":true,"message":"Envio gerado com sucesso"}}`
	c := newClient(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(reply)) }, store)
	if err := c.Generate(context.Background(), "abc"); err != nil {
		t.Fatalf("formato da documentação: %v", err)
	}
	reply = `{"abc":{"status":false,"message":"Agência obrigatória"}}`
	if err := c.Generate(context.Background(), "abc"); err == nil || !strings.Contains(err.Error(), "Agência obrigatória") {
		t.Fatalf("status false: %v", err)
	}
	reply = `{"abc":"Etiqueta sem saldo","outro":"x"}`
	if err := c.Generate(context.Background(), "abc"); err == nil || !strings.Contains(err.Error(), "Etiqueta sem saldo") {
		t.Fatalf("mensagem em texto na etiqueta: %v", err)
	}
	reply = `{"error":"Para esta transportadora, informe a agência"}`
	if err := c.Generate(context.Background(), "abc"); err == nil || !strings.Contains(err.Error(), "informe a agência") {
		t.Fatalf("mensagem solta: %v", err)
	}
}

// Formato visto no sandbox real: status continua "released" depois de gerada.
func TestOrderGeneratedFromDetails(t *testing.T) {
	var o OrderInfo
	raw := `{"id":"a2dd","status":"released","tracking":null,"self_tracking":"ME26006ES26BR",
		"generated_at":"2026-09-30 02:52:20","canceled_at":null,
		"generated_key":{"finished_at":"2026-09-30 02:56:09","failed_at":null}}`
	if err := json.Unmarshal([]byte(raw), &o); err != nil {
		t.Fatal(err)
	}
	if !o.Generated() || o.Cancelled() || o.Code() != "ME26006ES26BR" {
		t.Fatalf("gerada=%v cancelada=%v código=%q", o.Generated(), o.Cancelled(), o.Code())
	}
	var pending OrderInfo
	_ = json.Unmarshal([]byte(`{"status":"released","generated_at":null,"generated_key":{"finished_at":null,"failed_at":null}}`), &pending)
	if pending.Generated() {
		t.Error("em geração não é gerada")
	}
	var failed OrderInfo
	_ = json.Unmarshal([]byte(`{"status":"released","generated_key":{"finished_at":"x","failed_at":"y"}}`), &failed)
	if failed.Generated() || !failed.GenerationFailed() {
		t.Error("geração com falha")
	}
}

// Visto no sandbox real: detalhes com "posted" enquanto o rastreio seguia em
// "released".
func TestOrderAsTracking(t *testing.T) {
	var o OrderInfo
	_ = json.Unmarshal([]byte(`{"id":"a2dd","status":"posted","posted_at":"2026-09-30 03:10:09","self_tracking":"ME26006ES26BR"}`), &o)
	tr := o.AsTracking()
	if tr.Status != "posted" || tr.PostedAt == nil || tr.Code() != "ME26006ES26BR" {
		t.Fatalf("rastreio a partir dos detalhes: %+v", tr)
	}
	var c OrderInfo
	_ = json.Unmarshal([]byte(`{"id":"x","status":"released","canceled_at":"2026-09-30 04:00:00"}`), &c)
	if c.AsTracking().Status != "canceled" {
		t.Fatalf("cancelada pelos detalhes: %+v", c.AsTracking())
	}
}
