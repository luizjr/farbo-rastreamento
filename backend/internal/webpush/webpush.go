// Package webpush entrega notificações ao navegador pelo protocolo Web Push:
// o conteúdo vai cifrado para o aparelho (RFC 8291, aes128gcm) e o servidor
// se identifica ao serviço de push com VAPID (RFC 8292).
//
// Só a biblioteca padrão: ECDH P-256, HKDF e AES-GCM. O teste confere o
// resultado contra os valores do apêndice A da RFC 8291.
package webpush

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// recordSize é o tamanho de registro anunciado no cabeçalho; a mensagem
// cabe num registro só (o limite do protocolo é 4096 bytes).
const recordSize = 4096

// MaxPayload é o maior conteúdo que cabe num registro, descontados o
// cabeçalho do aes128gcm, o delimitador e a etiqueta do GCM.
const MaxPayload = recordSize - 86 - 1 - 16

var (
	// ErrGone: a inscrição não existe mais (o usuário desinstalou o app,
	// revogou a permissão ou ela expirou). Deve ser apagada.
	ErrGone = errors.New("inscrição de push não existe mais")
	// ErrEndpoint: endereço que não é de um serviço de push conhecido.
	ErrEndpoint = errors.New("endereço de push não permitido")
)

// Subscription é o que o navegador devolve em pushManager.subscribe().
type Subscription struct {
	Endpoint string
	// P256dh é a chave pública do navegador (ponto P-256 não comprimido,
	// base64url); Auth, o segredo de autenticação de 16 bytes.
	P256dh string
	Auth   string
}

// VAPID é o par de chaves que identifica este servidor aos serviços de push.
type VAPID struct {
	private *ecdsa.PrivateKey
	public  []byte // ponto não comprimido, 65 bytes
	Subject string // mailto: ou https:
}

// GenerateVAPID cria um par novo.
func GenerateVAPID(subject string) (*VAPID, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	return newVAPID(key, subject)
}

func newVAPID(key *ecdsa.PrivateKey, subject string) (*VAPID, error) {
	pub, err := key.PublicKey.ECDH()
	if err != nil {
		return nil, err
	}
	return &VAPID{private: key, public: pub.Bytes(), Subject: subject}, nil
}

// ParseVAPID lê a chave privada (32 bytes, base64url) no formato usado pelas
// bibliotecas de Web Push; a pública é derivada dela.
func ParseVAPID(privateKey, subject string) (*VAPID, error) {
	raw, err := decodeB64(privateKey)
	if err != nil || len(raw) != 32 {
		return nil, fmt.Errorf("chave privada VAPID inválida (esperado base64url de 32 bytes)")
	}
	priv, err := ecdh.P256().NewPrivateKey(raw)
	if err != nil {
		return nil, fmt.Errorf("chave privada VAPID inválida: %w", err)
	}
	pub := priv.PublicKey().Bytes()
	key := &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{Curve: elliptic.P256(),
			X: new(big.Int).SetBytes(pub[1:33]), Y: new(big.Int).SetBytes(pub[33:])},
		D: new(big.Int).SetBytes(raw),
	}
	return newVAPID(key, subject)
}

// PublicKey é a applicationServerKey que o navegador precisa para se inscrever.
func (v *VAPID) PublicKey() string { return base64.RawURLEncoding.EncodeToString(v.public) }

// PrivateKey é a chave privada em base64url (para guardar).
func (v *VAPID) PrivateKey() string {
	return base64.RawURLEncoding.EncodeToString(v.private.D.FillBytes(make([]byte, 32)))
}

// authorization monta o cabeçalho "vapid t=<JWT>, k=<chave pública>".
func (v *VAPID) authorization(endpoint string, now time.Time) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"aud": u.Scheme + "://" + u.Host,
		// A RFC permite até 24 h; 12 h dá folga para relógio adiantado.
		"exp": now.Add(12 * time.Hour).Unix(),
		"sub": v.Subject,
	})
	signed, err := token.SignedString(v.private)
	if err != nil {
		return "", err
	}
	return "vapid t=" + signed + ", k=" + v.PublicKey(), nil
}

// ---------------------------------------------------------------------------
// Cifragem (RFC 8291)
// ---------------------------------------------------------------------------

// encrypt cifra o conteúdo para o navegador. asKey (chave efêmera do
// servidor) e salt vêm de fora só para o teste com os valores da RFC.
func encrypt(plaintext []byte, sub Subscription, asKey *ecdh.PrivateKey, salt []byte) ([]byte, error) {
	if len(plaintext) > MaxPayload {
		return nil, fmt.Errorf("conteúdo de push grande demais: %d bytes (máximo %d)", len(plaintext), MaxPayload)
	}
	uaRaw, err := decodeB64(sub.P256dh)
	if err != nil {
		return nil, fmt.Errorf("chave p256dh inválida: %w", err)
	}
	uaPublic, err := ecdh.P256().NewPublicKey(uaRaw)
	if err != nil {
		return nil, fmt.Errorf("chave p256dh inválida: %w", err)
	}
	authSecret, err := decodeB64(sub.Auth)
	if err != nil || len(authSecret) != 16 {
		return nil, fmt.Errorf("segredo auth inválido")
	}
	if len(salt) != 16 {
		return nil, fmt.Errorf("salt precisa de 16 bytes")
	}

	ecdhSecret, err := asKey.ECDH(uaPublic)
	if err != nil {
		return nil, err
	}
	asPublic := asKey.PublicKey().Bytes()

	// PRK_key = HKDF-Extract(auth_secret, ecdh_secret)
	// IKM = HKDF-Expand(PRK_key, "WebPush: info" || 0x00 || ua_public || as_public, 32)
	prkKey, err := hkdf.Extract(sha256.New, ecdhSecret, authSecret)
	if err != nil {
		return nil, err
	}
	keyInfo := "WebPush: info\x00" + string(uaRaw) + string(asPublic)
	ikm, err := hkdf.Expand(sha256.New, prkKey, keyInfo, 32)
	if err != nil {
		return nil, err
	}

	// PRK = HKDF-Extract(salt, IKM); CEK e NONCE saem dele.
	prk, err := hkdf.Extract(sha256.New, ikm, salt)
	if err != nil {
		return nil, err
	}
	cek, err := hkdf.Expand(sha256.New, prk, "Content-Encoding: aes128gcm\x00", 16)
	if err != nil {
		return nil, err
	}
	nonce, err := hkdf.Expand(sha256.New, prk, "Content-Encoding: nonce\x00", 12)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Cabeçalho: salt(16) || rs(4) || idlen(1) || keyid (a chave pública efêmera).
	header := make([]byte, 0, 86)
	header = append(header, salt...)
	header = binary.BigEndian.AppendUint32(header, recordSize)
	header = append(header, byte(len(asPublic)))
	header = append(header, asPublic...)

	// Registro único: o delimitador 0x02 marca o último registro.
	record := append(append([]byte{}, plaintext...), 0x02)
	return gcm.Seal(header, nonce, record, nil), nil
}

// ---------------------------------------------------------------------------
// Envio
// ---------------------------------------------------------------------------

// Urgência (RFC 8030 §5.3): muda como o aparelho economiza bateria.
const (
	UrgencyNormal = "normal"
	UrgencyHigh   = "high"
)

// Message é uma entrega.
type Message struct {
	Payload []byte
	// TTL: por quanto tempo o serviço de push guarda a mensagem se o
	// aparelho estiver desligado.
	TTL     time.Duration
	Urgency string
	// Topic substitui uma mensagem ainda não entregue com o mesmo tópico
	// (até 32 caracteres do alfabeto base64url).
	Topic string
}

// Client envia para os serviços de push.
type Client struct {
	vapid *VAPID
	http  *http.Client
	// allowed são os serviços aceitos; ver AllowedEndpoint.
	extraHosts []string
	now        func() time.Time
}

// NewClient cria o cliente. extraHosts acrescenta serviços de push à lista
// conhecida (ex.: um serviço de teste local).
func NewClient(vapid *VAPID, extraHosts []string) *Client {
	c := &Client{vapid: vapid, extraHosts: extraHosts, now: time.Now}
	c.http = &http.Client{
		Timeout: 15 * time.Second,
		// Redirecionamento poderia levar a um destino fora da lista.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	return c
}

// VAPID devolve as chaves em uso.
func (c *Client) VAPID() *VAPID { return c.vapid }

// knownServices são os serviços de push dos navegadores.
var knownServices = []string{
	"fcm.googleapis.com",                // Chrome, Edge e Android
	"android.googleapis.com",            // endereços antigos do Chrome
	"updates.push.services.mozilla.com", // Firefox
	"push.services.mozilla.com",
	".notify.windows.com", // Edge legado / Windows
	".push.apple.com",     // Safari (macOS e iOS 16.4+)
}

// AllowedEndpoint diz se o endereço é de um serviço de push conhecido.
//
// O backend faz um POST para esse endereço, que vem do navegador do
// cliente: sem esta lista, qualquer cliente poderia apontar o servidor para
// a rede interna (SSRF). Fora da lista, só os hosts de extraHosts — e esses
// aceitam http apenas quando são o próprio computador (serviço de teste).
func (c *Client) AllowedEndpoint(endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || u.User != nil {
		return ErrEndpoint
	}
	host := strings.ToLower(u.Hostname())
	for _, extra := range c.extraHosts {
		if strings.EqualFold(u.Host, extra) {
			if u.Scheme == "https" || (u.Scheme == "http" && isLoopback(host)) {
				return nil
			}
			return ErrEndpoint
		}
	}
	if u.Scheme != "https" || (u.Port() != "" && u.Port() != "443") {
		return ErrEndpoint
	}
	for _, service := range knownServices {
		if strings.HasPrefix(service, ".") {
			if strings.HasSuffix(host, service) {
				return nil
			}
		} else if host == service {
			return nil
		}
	}
	return ErrEndpoint
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Send cifra e entrega uma mensagem. Devolve ErrGone quando a inscrição não
// existe mais.
func (c *Client) Send(ctx context.Context, sub Subscription, msg Message) error {
	if err := c.AllowedEndpoint(sub.Endpoint); err != nil {
		return err
	}
	asKey, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	body, err := encrypt(msg.Payload, sub, asKey, salt)
	if err != nil {
		return err
	}
	auth, err := c.vapid.authorization(sub.Endpoint, c.now())
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.Endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	ttl := msg.TTL
	if ttl <= 0 {
		ttl = 12 * time.Hour
	}
	req.Header.Set("TTL", strconv.Itoa(int(ttl.Seconds())))
	req.Header.Set("Content-Encoding", "aes128gcm")
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Authorization", auth)
	if msg.Urgency != "" {
		req.Header.Set("Urgency", msg.Urgency)
	}
	if msg.Topic != "" {
		req.Header.Set("Topic", msg.Topic)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	detail, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	switch {
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		return ErrGone
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	default:
		return fmt.Errorf("serviço de push respondeu %d: %s", resp.StatusCode, strings.TrimSpace(string(detail)))
	}
}

func decodeB64(v string) ([]byte, error) {
	v = strings.TrimRight(strings.TrimSpace(v), "=")
	v = strings.NewReplacer("+", "-", "/", "_").Replace(v)
	return base64.RawURLEncoding.DecodeString(v)
}
