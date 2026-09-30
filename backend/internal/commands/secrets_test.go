package commands

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/leakcheck"
	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/protocols"
	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/protocols/gt06"
)

// Credenciais sintéticas (issue #1). A APN fica em ASCII imprimível porque
// aqui ela viaja dentro de um comando (override e texto livre só aceitam
// esse intervalo), mas ainda com o que muda de forma quando escapado.
const (
	commandSecret = "Zq7Kx2Wm9Rt4"
	apnSecret     = `Qb"<k>&\'+/=7x`
)

func withSecrets(h *harness) {
	h.device.CommandPassword = commandSecret
	h.device.APNPassword = apnSecret
	h.device.CommandOverrides["ENGINE_RESUME"] = "HFYD," + commandSecret + "#"
}

// assertClean confere que nada do que sai do serviço — registro gravado,
// auditoria e eventos do WebSocket — carrega as credenciais.
func assertClean(t *testing.T, h *harness) {
	t.Helper()
	check := func(label string, v any) {
		body, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if found := leakcheck.Find(body, commandSecret, apnSecret); len(found) > 0 {
			t.Errorf("%s vazou credencial: %v\n%s", label, found, body)
		}
	}

	h.store.mu.Lock()
	for id, cmd := range h.store.commands {
		check("registro "+id.String(), cmd)
	}
	h.store.mu.Unlock()

	h.audit.mu.Lock()
	for _, entry := range h.audit.entries {
		check("auditoria "+entry.Action, entry)
	}
	h.audit.mu.Unlock()

	h.publisher.mu.Lock()
	for i, data := range h.publisher.data {
		check("evento "+h.publisher.topics[i], map[string]any{"type": h.publisher.topics[i], "data": data})
	}
	h.publisher.mu.Unlock()
}

func TestCommandPayloadIsRedactedButDeviceGetsPassword(t *testing.T) {
	h := newHarness(t, stoppedVehicle(), nil)
	withSecrets(h)
	ctx := context.Background()

	cases := []struct {
		req      Request
		wantText string // o que o aparelho precisa receber
	}{
		{Request{Type: protocols.CommandEngineCut}, "DYD," + commandSecret + "#"},
		{Request{Type: protocols.CommandEngineResume}, "HFYD," + commandSecret + "#"},
		{Request{Type: protocols.CommandSetInterval, Params: map[string]string{"seconds": "30"}},
			"TIMER," + commandSecret + ",30#"},
		// Texto livre do admin com a senha APN no meio.
		{Request{Type: protocols.CommandCustom, RawOverride: "APN,zap.vivo.com.br,user," + apnSecret + "#"},
			"APN,zap.vivo.com.br,user," + apnSecret + "#"},
	}
	for _, tc := range cases {
		tc.req.Device = h.device
		cmd, err := h.service.Send(ctx, tc.req)
		if err != nil {
			t.Fatalf("%s: %v", tc.req.Type, err)
		}
		if cmd.Status != StatusSent {
			t.Fatalf("%s: esperava SENT, veio %q", tc.req.Type, cmd.Status)
		}

		// O aparelho recebe o pacote real, com a senha.
		_, text, err := gt06.DecodeServerCommand(h.sender.last())
		if err != nil {
			t.Fatal(err)
		}
		if text != tc.wantText {
			t.Errorf("%s: o aparelho recebeu %q, esperava %q", tc.req.Type, text, tc.wantText)
		}
		// O que volta para a API não.
		if !strings.Contains(cmd.Payload, "***") {
			t.Errorf("%s: payload sem o marcador de redação: %q", tc.req.Type, cmd.Payload)
		}
	}
	assertClean(t, h)
}

func TestAckEchoingPasswordIsRedacted(t *testing.T) {
	h := newHarness(t, stoppedVehicle(), nil)
	withSecrets(h)
	ctx := context.Background()

	ok, err := h.service.Send(ctx, Request{Device: h.device, Type: protocols.CommandRequestStatus})
	if err != nil {
		t.Fatal(err)
	}
	failed, err := h.service.Send(ctx, Request{Device: h.device, Type: protocols.CommandEngineCut})
	if err != nil {
		t.Fatal(err)
	}

	// Firmware que ecoa o comando — em caixa alta, para não bastar comparar
	// o texto exato.
	h.service.HandleAck(ctx, h.device, nil, ok.CorrelationKey,
		"STATUS,"+strings.ToUpper(commandSecret)+"#: Battery:83% GSM:4", true)
	h.service.HandleAck(ctx, h.device, nil, failed.CorrelationKey,
		"DYD,"+commandSecret+"#: Fail! Speed too high", false)
	// Resposta sem comando aberto só vai para o log, mas também redigida.
	h.service.HandleAck(ctx, h.device, nil, 0, "PWD "+commandSecret+" ERR", true)

	acked, _ := h.store.Get(ctx, ok.ID)
	if acked.Status != StatusAcknowledged || acked.Response != "STATUS,***#: Battery:83% GSM:4" {
		t.Errorf("resposta gravada: %q (%s)", acked.Response, acked.Status)
	}
	rejected, _ := h.store.Get(ctx, failed.ID)
	if rejected.Status != StatusFailed || !strings.Contains(rejected.Error, "DYD,***#") {
		t.Errorf("falha gravada: %q (%s)", rejected.Error, rejected.Status)
	}
	assertClean(t, h)
}

func TestHistoryIsRedactedOnRead(t *testing.T) {
	// Registro gravado por um backend antigo (antes da redação) continua
	// saindo limpo: ListByDevice redige com as credenciais atuais.
	h := newHarness(t, stoppedVehicle(), nil)
	withSecrets(h)
	ctx := context.Background()

	legacy := &Command{
		DeviceID: h.device.ID, Command: string(protocols.CommandEngineCut), Status: StatusAcknowledged,
		Payload:  `xx\x11\x80\x0F\x00\x00\x00\x01DYD,` + commandSecret + `#\x00\x02`,
		Response: "DYD," + strings.ToLower(commandSecret) + "#=Success",
	}
	if err := h.store.Create(ctx, legacy); err != nil {
		t.Fatal(err)
	}

	list, err := h.service.ListByDevice(ctx, h.device, 10)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(list)
	if found := leakcheck.Find(body, commandSecret, apnSecret); len(found) > 0 {
		t.Fatalf("histórico vazou credencial: %v\n%s", found, body)
	}
	if !strings.Contains(list[0].Payload, "DYD,***#") {
		t.Fatalf("payload legado: %q", list[0].Payload)
	}
}
