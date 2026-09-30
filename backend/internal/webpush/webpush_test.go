package webpush

import (
	"context"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/base64"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func b64(t *testing.T, s string) []byte {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(strings.Join(strings.Fields(s), ""))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// Exemplo da seção 5 / apêndice A da RFC 8291.
func TestEncryptMatchesRFC8291Example(t *testing.T) {
	asKey, err := ecdh.P256().NewPrivateKey(b64(t, "yfWPiYE-n46HLnH0KqZOF1fJJU3MYrct3AELtAQ-oRw"))
	if err != nil {
		t.Fatal(err)
	}
	if got := base64.RawURLEncoding.EncodeToString(asKey.PublicKey().Bytes()); got !=
		"BP4z9KsN6nGRTbVYI_c7VJSPQTBtkgcy27mlmlMoZIIgDll6e3vCYLocInmYWAmS6TlzAC8wEqKK6PBru3jl7A8" {
		t.Fatalf("chave pública do servidor diferente da RFC: %s", got)
	}
	sub := Subscription{
		Endpoint: "https://push.example.net/push/JzLQ3raZJfFBR0aqvOMsLrt54w4rJUsV",
		P256dh:   "BCVxsr7N_eNgVRqvHtD0zTZsEc6-VV-JvLexhqUzORcxaOzi6-AYWXvTBHm4bjyPjs7Vd8pZGH6SRpkNtoIAiw4",
		Auth:     "BTBZMqHH6r4Tts7J_aSIgg",
	}
	body, err := encrypt([]byte("When I grow up, I want to be a watermelon"), sub, asKey, b64(t, "DGv6ra1nlYgDCS1FRnbzlw"))
	if err != nil {
		t.Fatal(err)
	}
	want := `DGv6ra1nlYgDCS1FRnbzlwAAEABBBP4z9KsN6nGRTbVYI_c7VJSPQTBtkgcy27ml
		mlMoZIIgDll6e3vCYLocInmYWAmS6TlzAC8wEqKK6PBru3jl7A_yl95bQpu6cVPT
		pK4Mqgkf1CXztLVBSt2Ks3oZwbuwXPXLWyouBWLVWGNWQexSgSxsj_Qulcy4a-fN`
	if got := base64.RawURLEncoding.EncodeToString(body); got != strings.Join(strings.Fields(want), "") {
		t.Fatalf("corpo cifrado diferente do exemplo da RFC:\n got %s", got)
	}
	// O texto da RFC diz "Content-Length: 145", mas o corpo do próprio
	// exemplo tem 144 bytes: 86 de cabeçalho + 41 do texto + 1 delimitador
	// + 16 da etiqueta do GCM.
	if len(body) != 144 {
		t.Errorf("tamanho do corpo: esperado 144, veio %d", len(body))
	}
}

func TestEncryptRejectsBadInput(t *testing.T) {
	asKey, _ := ecdh.P256().GenerateKey(nil)
	good := Subscription{P256dh: "BCVxsr7N_eNgVRqvHtD0zTZsEc6-VV-JvLexhqUzORcxaOzi6-AYWXvTBHm4bjyPjs7Vd8pZGH6SRpkNtoIAiw4", Auth: "BTBZMqHH6r4Tts7J_aSIgg"}
	salt := make([]byte, 16)
	for name, sub := range map[string]Subscription{
		"p256dh que não é ponto": {P256dh: "AAAA", Auth: good.Auth},
		"auth curto":             {P256dh: good.P256dh, Auth: "AAAA"},
	} {
		if _, err := encrypt([]byte("x"), sub, asKey, salt); err == nil {
			t.Errorf("%s: devia falhar", name)
		}
	}
	if _, err := encrypt(make([]byte, MaxPayload+1), good, asKey, salt); err == nil {
		t.Error("conteúdo maior que um registro devia falhar")
	}
	if _, err := encrypt(make([]byte, MaxPayload), good, asKey, salt); err != nil {
		t.Errorf("conteúdo no limite devia caber: %v", err)
	}
}

func TestVAPIDRoundTripAndSignature(t *testing.T) {
	v, err := GenerateVAPID("mailto:central@farbo.test")
	if err != nil {
		t.Fatal(err)
	}
	again, err := ParseVAPID(v.PrivateKey(), v.Subject)
	if err != nil || again.PublicKey() != v.PublicKey() {
		t.Fatalf("chave privada guardada não reproduz a pública: %v", err)
	}
	if len(b64(t, v.PublicKey())) != 65 {
		t.Error("a applicationServerKey é o ponto P-256 não comprimido (65 bytes)")
	}

	now := time.Unix(1_800_000_000, 0)
	header, err := v.authorization("https://fcm.googleapis.com/fcm/send/abc", now)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.SplitN(strings.TrimPrefix(header, "vapid "), ", ", 2)
	tokenPart, keyPart := strings.TrimPrefix(parts[0], "t="), strings.TrimPrefix(parts[1], "k=")
	if keyPart != v.PublicKey() {
		t.Fatal("k= precisa ser a chave pública VAPID")
	}
	pub := b64(t, keyPart)
	verifyKey := &ecdsa.PublicKey{Curve: elliptic.P256(), X: new(big.Int).SetBytes(pub[1:33]), Y: new(big.Int).SetBytes(pub[33:])}
	claims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(tokenPart, claims, func(*jwt.Token) (any, error) { return verifyKey, nil },
		jwt.WithValidMethods([]string{"ES256"}), jwt.WithTimeFunc(func() time.Time { return now }))
	if err != nil || !parsed.Valid {
		t.Fatalf("JWT VAPID não confere com a chave: %v", err)
	}
	if claims["aud"] != "https://fcm.googleapis.com" || claims["sub"] != "mailto:central@farbo.test" {
		t.Errorf("claims: %v", claims)
	}
	if exp, _ := claims.GetExpirationTime(); exp.Sub(now) > 24*time.Hour {
		t.Error("exp passa das 24 h que a RFC 8292 permite")
	}

	if _, err := ParseVAPID("curta", "x"); err == nil {
		t.Error("chave privada inválida devia ser recusada")
	}
}

func TestAllowedEndpointBlocksSSRF(t *testing.T) {
	v, _ := GenerateVAPID("mailto:x@farbo.test")
	c := NewClient(v, []string{"127.0.0.1:18199"})
	allowed := []string{
		"https://fcm.googleapis.com/fcm/send/abc:def",
		"https://updates.push.services.mozilla.com/wpush/v2/gAAA",
		"https://web.push.apple.com/QGuQ",
		"https://wns2-bn3p.notify.windows.com/w/?token=x",
		"http://127.0.0.1:18199/push/abc", // serviço de teste configurado
	}
	for _, e := range allowed {
		if err := c.AllowedEndpoint(e); err != nil {
			t.Errorf("%s devia ser aceito: %v", e, err)
		}
	}
	blocked := []string{
		"http://fcm.googleapis.com/fcm/send/abc",       // sem TLS
		"https://fcm.googleapis.com:8443/fcm/send/abc", // porta estranha
		"https://127.0.0.1/push",
		"http://localhost:8080/api/admin",
		"https://169.254.169.254/latest/meta-data",
		"https://fcm.googleapis.com.atacante.com/x",
		"https://atacantepush.apple.com/x",
		"https://user:senha@fcm.googleapis.com/x",
		"http://127.0.0.1:5432/",     // host de teste só na porta configurada
		"http://10.0.0.5:18199/push", // extra só vale no próprio computador e exato
		"ftp://fcm.googleapis.com/x",
		"not a url",
	}
	for _, e := range blocked {
		if err := c.AllowedEndpoint(e); !errors.Is(err, ErrEndpoint) {
			t.Errorf("%s devia ser recusado, veio %v", e, err)
		}
	}
}

func TestSendHeadersAndGoneSubscription(t *testing.T) {
	var got *http.Request
	var body []byte
	status := http.StatusCreated
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(status)
	}))
	defer srv.Close()

	v, _ := GenerateVAPID("mailto:x@farbo.test")
	c := NewClient(v, []string{strings.TrimPrefix(srv.URL, "http://")})
	ua, _ := ecdh.P256().GenerateKey(nil)
	sub := Subscription{Endpoint: srv.URL + "/push/abc",
		P256dh: base64.RawURLEncoding.EncodeToString(ua.PublicKey().Bytes()), Auth: "BTBZMqHH6r4Tts7J_aSIgg"}

	err := c.Send(context.Background(), sub, Message{Payload: []byte(`{"title":"x"}`), TTL: time.Hour,
		Urgency: UrgencyHigh, Topic: "sos-veiculo"})
	if err != nil {
		t.Fatal(err)
	}
	for header, want := range map[string]string{"TTL": "3600", "Content-Encoding": "aes128gcm",
		"Urgency": "high", "Topic": "sos-veiculo", "Content-Type": "application/octet-stream"} {
		if got.Header.Get(header) != want {
			t.Errorf("%s: %q, esperado %q", header, got.Header.Get(header), want)
		}
	}
	if !strings.HasPrefix(got.Header.Get("Authorization"), "vapid t=") {
		t.Errorf("Authorization: %q", got.Header.Get("Authorization"))
	}
	if len(body) < 86+16 || body[20] != 65 {
		t.Errorf("corpo sem o cabeçalho aes128gcm: %d bytes", len(body))
	}

	for _, code := range []int{http.StatusGone, http.StatusNotFound} {
		status = code
		if err := c.Send(context.Background(), sub, Message{Payload: []byte("x")}); !errors.Is(err, ErrGone) {
			t.Errorf("%d devia virar ErrGone, veio %v", code, err)
		}
	}
	status = http.StatusTooManyRequests
	if err := c.Send(context.Background(), sub, Message{Payload: []byte("x")}); err == nil || errors.Is(err, ErrGone) {
		t.Errorf("429 é erro temporário, não inscrição morta: %v", err)
	}
	if err := c.Send(context.Background(), Subscription{Endpoint: "https://evil.example/x"}, Message{}); !errors.Is(err, ErrEndpoint) {
		t.Errorf("endereço fora da lista não pode ser chamado: %v", err)
	}
}
