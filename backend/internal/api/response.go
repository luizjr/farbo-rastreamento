package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/database"
)

// maxBodyBytes limita o corpo aceito nas requisições (§27).
const maxBodyBytes = 1 << 20 // 1 MiB

type errorBody struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("falha ao escrever resposta JSON", "err", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorBody{Error: message})
}

// decodeJSON lê o corpo com limite de tamanho e recusa campos desconhecidos,
// que normalmente indicam erro de integração.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		return err
	}
	// Garante que não veio um segundo objeto JSON grudado no primeiro.
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("o corpo deve conter um único objeto JSON")
	}
	return nil
}

// urlUUID lê um parâmetro de rota no formato UUID.
func urlUUID(r *http.Request, name string) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, name))
}

// queryInt lê um inteiro da query string, com valor padrão.
func queryInt(r *http.Request, name string, def int) int {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return v
}

func queryBool(r *http.Request, name string, def bool) bool {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return def
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return def
	}
	return v
}

// queryTime aceita RFC3339 ou epoch em segundos.
func queryTime(r *http.Request, name string, def time.Time) (time.Time, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return def, nil
	}
	if ts, err := time.Parse(time.RFC3339, raw); err == nil {
		return ts.UTC(), nil
	}
	if epoch, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return time.Unix(epoch, 0).UTC(), nil
	}
	return time.Time{}, errors.New("data inválida em " + name + ": use RFC3339 ou epoch em segundos")
}

// handleStoreError traduz erros de repositório em respostas HTTP.
func handleStoreError(w http.ResponseWriter, err error, notFoundMessage string) {
	switch {
	case errors.Is(err, database.ErrNotFound):
		writeError(w, http.StatusNotFound, notFoundMessage)
	case errors.Is(err, database.ErrConflict):
		writeError(w, http.StatusConflict, "já existe um registro com esses dados")
	case errors.Is(err, database.ErrForeignKey):
		writeError(w, http.StatusBadRequest, "referência inválida em um dos campos")
	default:
		slog.Error("erro inesperado no repositório", "err", err)
		writeError(w, http.StatusInternalServerError, "erro interno")
	}
}
