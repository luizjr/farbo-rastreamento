package melhorenvio

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/database"
)

const provider = "melhorenvio"

// stateTTL é quanto tempo a pessoa tem para autorizar no Melhor Envios.
const stateTTL = 15 * time.Minute

// DBStore guarda o token no banco, cifrado com AES-GCM. A chave deriva de um
// segredo do servidor: quem lê só o banco não consegue usar o token.
type DBStore struct {
	db  *database.DB
	gcm cipher.AEAD
}

func NewDBStore(db *database.DB, secret string) (*DBStore, error) {
	key := sha256.Sum256([]byte("farbo:integration-tokens:" + secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &DBStore{db: db, gcm: gcm}, nil
}

func (s *DBStore) seal(plain string) (string, error) {
	nonce := make([]byte, s.gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return "v1:" + base64.StdEncoding.EncodeToString(s.gcm.Seal(nonce, nonce, []byte(plain), nil)), nil
}

func (s *DBStore) open(sealed string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(sealed, "v1:"))
	if err != nil || len(raw) < s.gcm.NonceSize() {
		return "", errors.New("token guardado ilegível")
	}
	plain, err := s.gcm.Open(nil, raw[:s.gcm.NonceSize()], raw[s.gcm.NonceSize():], nil)
	if err != nil {
		// Segredo do servidor trocado: é preciso conectar de novo.
		return "", errors.New("token guardado ilegível")
	}
	return string(plain), nil
}

func (s *DBStore) Load(ctx context.Context) (*Token, error) {
	var access, refresh string
	var expires time.Time
	err := database.MapError(s.db.QueryRow(ctx, `
		SELECT access_token, refresh_token, expires_at FROM integration_tokens WHERE provider = $1`, provider,
	).Scan(&access, &refresh, &expires))
	if errors.Is(err, database.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	t := &Token{ExpiresAt: expires}
	if t.AccessToken, err = s.open(access); err != nil {
		return nil, nil
	}
	if t.RefreshToken, err = s.open(refresh); err != nil {
		return nil, nil
	}
	return t, nil
}

func (s *DBStore) Save(ctx context.Context, t *Token) error {
	access, err := s.seal(t.AccessToken)
	if err != nil {
		return err
	}
	refresh, err := s.seal(t.RefreshToken)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(ctx, `
		INSERT INTO integration_tokens (provider, access_token, refresh_token, expires_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (provider) DO UPDATE SET access_token = EXCLUDED.access_token,
			refresh_token = EXCLUDED.refresh_token, expires_at = EXCLUDED.expires_at, updated_at = NOW()`,
		provider, access, refresh, t.ExpiresAt)
	return database.MapError(err)
}

func (s *DBStore) Delete(ctx context.Context) error {
	_, err := s.db.Exec(ctx, `DELETE FROM integration_tokens WHERE provider = $1`, provider)
	return database.MapError(err)
}

// NewState registra o "state" de um Conectar, ligado a quem clicou.
func (s *DBStore) NewState(ctx context.Context, userID uuid.UUID) (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	state := hex.EncodeToString(raw)
	// Aproveita para limpar os que ninguém usou.
	if _, err := s.db.Exec(ctx, `DELETE FROM integration_states WHERE created_at < NOW() - INTERVAL '1 day'`); err != nil {
		return "", database.MapError(err)
	}
	_, err := s.db.Exec(ctx, `INSERT INTO integration_states (state, provider, user_id) VALUES ($1, $2, $3)`,
		state, provider, userID)
	return state, database.MapError(err)
}

// ConsumeState vale uma vez só e dentro do prazo.
func (s *DBStore) ConsumeState(ctx context.Context, state string) (bool, error) {
	if state == "" {
		return false, nil
	}
	tag, err := s.db.Exec(ctx, `
		DELETE FROM integration_states WHERE state = $1 AND provider = $2 AND created_at > $3`,
		state, provider, time.Now().Add(-stateTTL))
	if err != nil {
		return false, database.MapError(err)
	}
	return tag.RowsAffected() == 1, nil
}
