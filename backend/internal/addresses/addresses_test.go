package addresses

import (
	"errors"
	"strings"
	"testing"
)

func valid() Address {
	return Address{
		ZipCode: "01310-100", Street: " Avenida Paulista ", Number: "1000", Complement: "apto 12",
		District: "Bela Vista", City: "São Paulo", State: "sp",
	}
}

func TestNormalizeCleansFields(t *testing.T) {
	got, err := Normalize(valid())
	if err != nil {
		t.Fatalf("endereço válido recusado: %v", err)
	}
	if got.ZipCode != "01310100" {
		t.Errorf("CEP = %q, quer só os dígitos", got.ZipCode)
	}
	if got.State != "SP" {
		t.Errorf("UF = %q, quer em maiúsculas", got.State)
	}
	if got.Street != "Avenida Paulista" {
		t.Errorf("rua = %q, quer sem espaços nas pontas", got.Street)
	}
}

func TestNormalizeAcceptsWithoutComplement(t *testing.T) {
	a := valid()
	a.Complement = ""
	a.Number = "S/N"
	if _, err := Normalize(a); err != nil {
		t.Fatalf("sem complemento e S/N deveria passar: %v", err)
	}
}

func TestNormalizeRejects(t *testing.T) {
	cases := map[string]func(*Address){
		"CEP curto":         func(a *Address) { a.ZipCode = "0131010" },
		"CEP longo":         func(a *Address) { a.ZipCode = "013101000" },
		"sem rua":           func(a *Address) { a.Street = "  " },
		"sem número":        func(a *Address) { a.Number = "" },
		"sem bairro":        func(a *Address) { a.District = "" },
		"sem cidade":        func(a *Address) { a.City = "" },
		"UF que não existe": func(a *Address) { a.State = "XX" },
		"UF vazia":          func(a *Address) { a.State = "" },
		"rua longa demais":  func(a *Address) { a.Street = strings.Repeat("a", 151) },
		"complemento longo": func(a *Address) { a.Complement = strings.Repeat("a", 81) },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			a := valid()
			change(&a)
			_, err := Normalize(a)
			var validation ValidationError
			if !errors.As(err, &validation) {
				t.Fatalf("esperava ValidationError, veio %v", err)
			}
		})
	}
}
