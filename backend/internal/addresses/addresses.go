// Package addresses guarda o endereço de entrega do cliente, para onde a
// central envia o rastreador contratado.
package addresses

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/database"
)

// Address é o endereço de entrega. Vai também, como cópia, em cada
// assinatura contratada (ver billing.Subscription).
type Address struct {
	// ZipCode é o CEP só com os 8 dígitos.
	ZipCode    string `json:"zipCode"`
	Street     string `json:"street"`
	Number     string `json:"number"`
	Complement string `json:"complement"`
	District   string `json:"district"`
	City       string `json:"city"`
	// State é a sigla da UF, em maiúsculas.
	State string `json:"state"`
}

// ValidationError descreve um endereço recusado; a mensagem vai para a tela.
type ValidationError struct{ Message string }

func (e ValidationError) Error() string { return e.Message }

var states = map[string]bool{
	"AC": true, "AL": true, "AP": true, "AM": true, "BA": true, "CE": true, "DF": true,
	"ES": true, "GO": true, "MA": true, "MT": true, "MS": true, "MG": true, "PA": true,
	"PB": true, "PR": true, "PE": true, "PI": true, "RJ": true, "RN": true, "RS": true,
	"RO": true, "RR": true, "SC": true, "SP": true, "SE": true, "TO": true,
}

// Normalize limpa os campos e confere as regras. Devolve o endereço pronto
// para gravar (CEP só com dígitos, UF em maiúsculas).
func Normalize(a Address) (Address, error) {
	a.Street = strings.TrimSpace(a.Street)
	a.Number = strings.TrimSpace(a.Number)
	a.Complement = strings.TrimSpace(a.Complement)
	a.District = strings.TrimSpace(a.District)
	a.City = strings.TrimSpace(a.City)
	a.State = strings.ToUpper(strings.TrimSpace(a.State))

	var zip strings.Builder
	for _, r := range a.ZipCode {
		if r >= '0' && r <= '9' {
			zip.WriteRune(r)
		}
	}
	a.ZipCode = zip.String()

	switch {
	case len(a.ZipCode) != 8:
		return a, ValidationError{"CEP inválido: informe os 8 dígitos"}
	case a.Street == "" || tooLong(a.Street, 150):
		return a, ValidationError{"informe a rua (até 150 caracteres)"}
	case a.Number == "" || tooLong(a.Number, 20):
		return a, ValidationError{"informe o número (ou S/N)"}
	case tooLong(a.Complement, 80):
		return a, ValidationError{"complemento: até 80 caracteres"}
	case a.District == "" || tooLong(a.District, 100):
		return a, ValidationError{"informe o bairro (até 100 caracteres)"}
	case a.City == "" || tooLong(a.City, 100):
		return a, ValidationError{"informe a cidade (até 100 caracteres)"}
	case !states[a.State]:
		return a, ValidationError{"UF inválida"}
	}
	return a, nil
}

func tooLong(s string, max int) bool { return utf8.RuneCountInString(s) > max }

// ---------------------------------------------------------------------------
// Repositório
// ---------------------------------------------------------------------------

type Repository struct{ db *database.DB }

func NewRepository(db *database.DB) *Repository { return &Repository{db: db} }

// Get devolve o endereço do cliente, ou nil se ele ainda não cadastrou.
func (r *Repository) Get(ctx context.Context, customerID uuid.UUID) (*Address, error) {
	return GetWith(ctx, r.db, customerID)
}

// GetWith lê pelo Querier informado (pool ou transação).
func GetWith(ctx context.Context, q database.Querier, customerID uuid.UUID) (*Address, error) {
	var a Address
	err := database.MapError(q.QueryRow(ctx, `
		SELECT zip_code, street, number, complement, district, city, state
		FROM customer_addresses WHERE customer_id = $1`, customerID,
	).Scan(&a.ZipCode, &a.Street, &a.Number, &a.Complement, &a.District, &a.City, &a.State))
	if errors.Is(err, database.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// Save grava (ou substitui) o endereço do cliente. Espera o endereço já
// normalizado.
func (r *Repository) Save(ctx context.Context, customerID uuid.UUID, a Address) (*Address, error) {
	_, err := r.db.Exec(ctx, `
		INSERT INTO customer_addresses (customer_id, zip_code, street, number, complement, district, city, state)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (customer_id) DO UPDATE SET
			zip_code = EXCLUDED.zip_code, street = EXCLUDED.street, number = EXCLUDED.number,
			complement = EXCLUDED.complement, district = EXCLUDED.district, city = EXCLUDED.city,
			state = EXCLUDED.state, updated_at = NOW()`,
		customerID, a.ZipCode, a.Street, a.Number, a.Complement, a.District, a.City, a.State)
	if err != nil {
		return nil, database.MapError(err)
	}
	return &a, nil
}
