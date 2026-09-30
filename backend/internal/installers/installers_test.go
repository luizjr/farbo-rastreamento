package installers

import (
	"errors"
	"strings"
	"testing"
)

func valid() Input {
	return Input{Name: "  Auto Elétrica do Zé ", City: "São Paulo - SP", WhatsApp: "(11) 99999-0000", ServesMoto: true}
}

func TestNormalizeCleansAndFormatsWhatsApp(t *testing.T) {
	cases := map[string]string{
		"(11) 99999-0000":     "5511999990000",
		"11 3333-4444":        "551133334444",
		"+55 (21) 98888-7777": "5521988887777",
		"55 11 99999-0000":    "5511999990000",
	}
	for raw, want := range cases {
		in := valid()
		in.WhatsApp = raw
		got, err := Normalize(in)
		if err != nil || got.WhatsApp != want {
			t.Errorf("WhatsApp %q → %q (%v), esperado %q", raw, got.WhatsApp, err, want)
		}
		if got.Name != "Auto Elétrica do Zé" {
			t.Errorf("nome não foi aparado: %q", got.Name)
		}
	}
}

func TestNormalizeRejects(t *testing.T) {
	cases := map[string]func(*Input){
		"sem nome":             func(i *Input) { i.Name = "  " },
		"WhatsApp curto":       func(i *Input) { i.WhatsApp = "9999-0000" },
		"WhatsApp estrangeiro": func(i *Input) { i.WhatsApp = "+1 415 555 0100" },
		"não atende nada":      func(i *Input) { i.ServesMoto, i.ServesCar = false, false },
		"valor negativo":       func(i *Input) { v := -1; i.PriceMotoCents = &v },
		"descrição longa":      func(i *Input) { i.Description = strings.Repeat("a", 301) },
		"celular sem o 9":      func(i *Input) { i.WhatsApp = "(11) 8999-00001" },
		"DDD com zero":         func(i *Input) { i.WhatsApp = "(01) 99999-0000" },
		"fixo começando em 8":  func(i *Input) { i.WhatsApp = "(11) 8333-4444" },
	}
	for name, mutate := range cases {
		in := valid()
		mutate(&in)
		var validation ValidationError
		if _, err := Normalize(in); !errors.As(err, &validation) {
			t.Errorf("%s: deveria ser recusado (err=%v)", name, err)
		}
	}
}

func TestPublicOmitsControlFields(t *testing.T) {
	price := 12000
	i := &Installer{Name: "Zé", WhatsApp: "5511999990000", ServesMoto: true, PriceMotoCents: &price, Active: true}
	p := i.Public()
	if p.Name != "Zé" || p.WhatsApp != "5511999990000" || p.PriceMotoCents == nil || *p.PriceMotoCents != 12000 {
		t.Fatalf("visão pública = %+v", p)
	}
}
