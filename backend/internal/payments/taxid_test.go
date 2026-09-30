package payments

import "testing"

func TestValidTaxID(t *testing.T) {
	cases := map[string]bool{
		"529.982.247-25":     true, // CPF válido
		"52998224725":        true,
		"529.982.247-24":     false, // dígito errado
		"111.111.111-11":     false, // repetido passa no cálculo, mas é inválido
		"11.222.333/0001-81": true,  // CNPJ válido
		"11222333000181":     true,
		"11.222.333/0001-80": false,
		"00.000.000/0000-00": false,
		"123":                false,
		"":                   false,
	}
	for doc, want := range cases {
		if got := validTaxID(doc); got != want {
			t.Errorf("validTaxID(%q) = %v, esperado %v", doc, got, want)
		}
	}
}

func TestCustomerForOnlyWithCompleteValidData(t *testing.T) {
	full := &Payer{Name: "Maria Silva", Email: "maria@x.com", Document: "529.982.247-25", Phone: "(11) 99999-0000"}
	c := customerFor(full)
	if c == nil || c.TaxID != "52998224725" || c.Cellphone != "11999990000" {
		t.Fatalf("pagador completo: %+v", c)
	}

	for name, payer := range map[string]*Payer{
		"sem pagador":     nil,
		"CPF inválido":    {Name: "Maria", Email: "maria@x.com", Document: "123.456.789-00", Phone: "11999990000"},
		"sem telefone":    {Name: "Maria", Email: "maria@x.com", Document: "529.982.247-25"},
		"sem nome":        {Email: "maria@x.com", Document: "529.982.247-25", Phone: "11999990000"},
		"e-mail inválido": {Name: "Maria", Email: "maria", Document: "529.982.247-25", Phone: "11999990000"},
	} {
		if customerFor(payer) != nil {
			t.Errorf("%s: o Pix deveria sair sem identificar o pagador", name)
		}
	}
}
