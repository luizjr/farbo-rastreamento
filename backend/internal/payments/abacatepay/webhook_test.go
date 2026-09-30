package abacatepay

import (
	"sort"
	"testing"
)

func TestVerifySignature(t *testing.T) {
	body := []byte(`{"id":"log_1","event":"transparent.completed","devMode":true,"data":{"id":"pix_char_1"}}`)
	signature := Sign(body, PublicWebhookKey)

	if !VerifySignature(body, signature, PublicWebhookKey) {
		t.Fatal("assinatura correta recusada")
	}
	if VerifySignature(body, "  "+signature+"  ", PublicWebhookKey) != true {
		t.Fatal("espaços em volta do cabeçalho não deveriam invalidar")
	}
	tampered := append([]byte{}, body...)
	tampered[len(tampered)-3] = 'X'
	if VerifySignature(tampered, signature, PublicWebhookKey) {
		t.Fatal("corpo alterado passou na verificação")
	}
	if VerifySignature(body, "", PublicWebhookKey) {
		t.Fatal("assinatura vazia passou")
	}
	if VerifySignature(body, Sign(body, "outra-chave"), PublicWebhookKey) {
		t.Fatal("assinatura com outra chave passou")
	}
}

func TestVerifySecret(t *testing.T) {
	if !VerifySecret("segredo", "segredo") {
		t.Fatal("segredo correto recusado")
	}
	if VerifySecret("errado", "segredo") || VerifySecret("", "segredo") {
		t.Fatal("segredo errado ou ausente aceito")
	}
	// Sem segredo configurado, nenhum webhook passa.
	if VerifySecret("", "") || VerifySecret("qualquer", "") {
		t.Fatal("sem segredo configurado o webhook deveria ser recusado")
	}
}

func TestParseEventAndChargeIDs(t *testing.T) {
	ev, err := ParseEvent([]byte(`{
		"id": "log_abc", "event": "transparent.completed", "apiVersion": 2, "devMode": true,
		"data": {
			"transparent": {"id": "pix_char_A", "status": "PAID", "amount": 6990},
			"payment": {"method": "PIX"},
			"history": [{"ref": "pix_char_B"}, {"ref": "pix_char_A"}],
			"customer": {"id": "cust_1", "name": "pix_char_ não é id completo? é sim"}
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if ev.ID != "log_abc" || ev.Event != "transparent.completed" || !ev.DevMode {
		t.Fatalf("evento = %+v", ev)
	}
	ids := ev.ChargeIDs()
	sort.Strings(ids)
	if len(ids) != 3 || ids[0] != "pix_char_ não é id completo? é sim" || ids[1] != "pix_char_A" || ids[2] != "pix_char_B" {
		// Qualquer texto que comece com pix_char_ é candidato: o que não
		// existir no banco é simplesmente ignorado.
		t.Fatalf("ids = %v", ids)
	}

	if _, err := ParseEvent([]byte(`{"event":"x"}`)); err == nil {
		t.Fatal("evento sem id deveria ser recusado")
	}
	if _, err := ParseEvent([]byte(`not json`)); err == nil {
		t.Fatal("corpo inválido deveria ser recusado")
	}
}
