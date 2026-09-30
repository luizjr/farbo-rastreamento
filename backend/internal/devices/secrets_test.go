package devices

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/leakcheck"
	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/protocols"
	"github.com/pedrofarbo/farbo-rastreamento/backend/internal/protocols/gt06"
)

// Credenciais sintéticas, diferentes entre si: a de comando só aceita letras
// e números; a APN leva o que muda de forma quando escapado (aspas, barra,
// HTML, URL) e um caractere não-ASCII.
const (
	commandSecret = "Zq7Kx2Wm9Rt4"
	apnSecret     = `Ap"<n>&\é'9+/=`
	apnUser       = "usuario-apn-7Hq"
)

func deviceWithSecrets() *Device {
	port, interval := 5023, 30
	seen := time.Now()
	return &Device{
		ID: uuid.New(), IMEI: "869247061230001", Model: "J16", Manufacturer: "Concox",
		Protocol: gt06.Name, Firmware: "1.0", PhoneNumber: "+5511999990000",
		Status: StatusOnline, LastSeenAt: &seen,
		APN: "zap.vivo.com.br", APNUser: apnUser, APNPassword: apnSecret,
		ServerHost: "rastreio.exemplo.com", ServerPort: &port, ReportIntervalSeconds: &interval,
		CommandPassword: commandSecret,
		CommandOverrides: map[string]string{
			"ENGINE_RESUME": "HFYD," + commandSecret + "#",
			"REBOOT":        "RESET#",
		},
		Notes: "instalado sob o painel",
	}
}

func assertNoLeak(t *testing.T, label string, v any) []byte {
	t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if found := leakcheck.Find(body, commandSecret, apnSecret); len(found) > 0 {
		t.Errorf("%s vazou credencial: %v\n%s", label, found, body)
	}
	return body
}

func TestDeviceJSONNeverCarriesPasswords(t *testing.T) {
	// Defesa em profundidade: mesmo serializado por engano, o cadastro não
	// leva senha nem override.
	body := assertNoLeak(t, "Device", deviceWithSecrets())
	for _, key := range []string{"apnPassword", "commandPassword", "commandOverrides"} {
		if strings.Contains(string(body), `"`+key+`"`) {
			t.Errorf("Device serializou %q", key)
		}
	}
}

func TestViewPerAudience(t *testing.T) {
	dev := deviceWithSecrets()

	admin := dev.View(AudienceAdmin)
	assertNoLeak(t, "View admin", admin)
	if !admin.APNPasswordSet || !admin.CommandPasswordSet {
		t.Error("o admin precisa saber que as senhas estão definidas")
	}
	if admin.APNUser != apnUser || admin.ServerHost == "" || admin.ServerPort == nil {
		t.Error("o admin edita o cadastro: APN, usuário e servidor precisam vir")
	}
	if got := admin.CommandOverrides["ENGINE_RESUME"]; got != "HFYD,***#" {
		t.Errorf("override com a senha deveria vir redigido, veio %q", got)
	}
	if got := admin.CommandOverrides["REBOOT"]; got != "RESET#" {
		t.Errorf("override sem senha vem inteiro, veio %q", got)
	}

	for name, audience := range map[string]Audience{"staff": AudienceStaff, "customer": AudienceCustomer} {
		v := dev.View(audience)
		body := assertNoLeak(t, "View "+name, v)
		if strings.Contains(string(body), apnUser) {
			t.Errorf("%s recebeu o usuário APN", name)
		}
		if v.APN != "" || v.ServerHost != "" || v.ServerPort != nil || v.ReportIntervalSeconds != nil ||
			len(v.CommandOverrides) != 0 || v.APNPasswordSet || v.CommandPasswordSet {
			t.Errorf("%s recebeu configuração do aparelho: %s", name, body)
		}
		if v.IMEI != dev.IMEI || v.Status != dev.Status || v.LastSeenAt == nil {
			t.Errorf("%s precisa da identificação e da situação para o painel", name)
		}
	}
	if v := dev.View(AudienceCustomer); v.Notes != "" || v.PhoneNumber != "" {
		t.Error("anotações e linha são da central, não do cliente")
	}
	if v := dev.View(AudienceStaff); v.Notes == "" || v.PhoneNumber == "" {
		t.Error("a equipe vê anotações e a linha do chip")
	}
}

// View é lista fechada: campo com "password" no nome só existe como
// indicador booleano. Quem acrescentar um campo de senha aqui quebra o teste.
func TestViewHasNoPasswordField(t *testing.T) {
	typ := reflect.TypeOf(View{})
	for i := range typ.NumField() {
		field := typ.Field(i)
		name := strings.ToLower(field.Name + field.Tag.Get("json"))
		if strings.Contains(name, "password") && field.Type.Kind() != reflect.Bool {
			t.Errorf("View.%s pode carregar senha", field.Name)
		}
	}
}

func TestRedactGT06Frame(t *testing.T) {
	proto := gt06.New(false)
	for _, cmd := range []protocols.Command{
		{Type: protocols.CommandEngineCut, Password: commandSecret, CorrelationKey: 0x31323334},
		{Type: protocols.CommandSetInterval, Password: commandSecret, Params: map[string]string{"seconds": "30"}},
		// O comando livre digitado pelo admin também pode trazer a senha,
		// inclusive em outra caixa.
		{Type: protocols.CommandCustom, Raw: "APN,zap," + strings.ToUpper(commandSecret) + "#"},
	} {
		frame, err := proto.EncodeCommand(cmd)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(strings.ToLower(string(frame)), strings.ToLower(commandSecret)) {
			t.Fatalf("%s: o pacote real precisa levar a senha", cmd.Type)
		}
		redacted := RedactBytes(frame, []string{commandSecret, apnSecret})
		text := printableCommand(redacted)
		assertNoLeak(t, string(cmd.Type), map[string]string{"payload": text})
		if !strings.Contains(text, Redacted) {
			t.Errorf("%s: esperava %s no lugar da senha: %q", cmd.Type, Redacted, text)
		}
	}
}

func TestRedactTextKeepsTextWithoutSecret(t *testing.T) {
	if got := RedactText("DYD=Success!", []string{commandSecret}); got != "DYD=Success!" {
		t.Fatalf("texto sem senha alterado: %q", got)
	}
	if got := RedactText("DYD,"+commandSecret+"#:ok "+strings.ToLower(commandSecret), []string{commandSecret}); got != "DYD,***#:ok ***" {
		t.Fatalf("redação incompleta: %q", got)
	}
	if got := RedactText("x", nil); got != "x" {
		t.Fatalf("sem segredo nada muda: %q", got)
	}
}

func TestPasswordsAreWriteOnly(t *testing.T) {
	current := deviceWithSecrets()

	// Campo vazio (a tela nunca recebe a senha para devolver) mantém a atual.
	kept, err := mergeInput(current, Input{IMEI: current.IMEI})
	if err != nil {
		t.Fatal(err)
	}
	if kept.APNPassword != apnSecret || kept.CommandPassword != commandSecret {
		t.Error("senha vazia deveria manter a atual")
	}
	if !reflect.DeepEqual(kept.CommandOverrides, current.CommandOverrides) {
		t.Error("overrides ausentes deveriam ficar como estão")
	}

	replaced, err := mergeInput(current, Input{APNPassword: "nova-apn", CommandPassword: "Nova123"})
	if err != nil {
		t.Fatal(err)
	}
	if replaced.APNPassword != "nova-apn" || replaced.CommandPassword != "Nova123" {
		t.Error("senha informada deveria substituir a atual")
	}

	cleared, err := mergeInput(current, Input{ClearAPNPassword: true, ClearCommandPassword: true,
		CommandOverrides: map[string]string{}})
	if err != nil {
		t.Fatal(err)
	}
	if cleared.APNPassword != "" || cleared.CommandPassword != "" || len(cleared.CommandOverrides) != 0 {
		t.Error("Clear* e overrides vazios deveriam apagar")
	}

	if _, err := mergeInput(current, Input{CommandPassword: "Outra1", ClearCommandPassword: true}); err == nil {
		t.Error("senha nova junto com apagar é ambíguo e deveria ser recusado")
	}
}

func TestRedactedOverrideRoundTrip(t *testing.T) {
	current := deviceWithSecrets()
	shown := current.View(AudienceAdmin).CommandOverrides

	// O admin reenvia o que a leitura mostrou: vale o texto original.
	merged, err := mergeInput(current, Input{CommandOverrides: shown})
	if err != nil {
		t.Fatal(err)
	}
	if merged.CommandOverrides["ENGINE_RESUME"] != "HFYD,"+commandSecret+"#" {
		t.Errorf("override redigido reenviado deveria manter o original: %q",
			merged.CommandOverrides["ENGINE_RESUME"])
	}

	// *** num texto diferente iria literalmente para o aparelho: recusado.
	if _, err := mergeInput(current, Input{CommandOverrides: map[string]string{
		"ENGINE_RESUME": "RELAY,***,0#",
	}}); err == nil {
		t.Error("override novo com *** deveria ser recusado")
	}
	if _, err := mergeInput(nil, Input{CommandOverrides: map[string]string{"ENGINE_CUT": "DYD,***#"}}); err == nil {
		t.Error("na criação não há texto original a manter")
	}
}

func TestProvisioningCommandsAreRedacted(t *testing.T) {
	svc := NewService(nil, protocols.NewRegistry(gt06.New(false)))
	dev := deviceWithSecrets()
	heartbeat := 300
	dev.HeartbeatIntervalSeconds = &heartbeat

	list := svc.ProvisioningCommands(dev)
	if len(list) != 3 {
		t.Fatalf("esperava servidor, intervalo e heartbeat, veio %+v", list)
	}
	assertNoLeak(t, "provisionamento", list)
	for _, cmd := range list {
		if !cmd.Available {
			t.Fatalf("%s indisponível: %s", cmd.Type, cmd.Reason)
		}
		// Só o TIMER leva senha no GT06.
		wantRedacted := cmd.Type == protocols.CommandSetInterval
		if cmd.Redacted != wantRedacted {
			t.Errorf("%s: redacted=%v, texto %q", cmd.Type, cmd.Redacted, cmd.Text)
		}
		if wantRedacted && !strings.Contains(cmd.Text, "TIMER,***,30#") {
			t.Errorf("TIMER deveria mostrar *** no lugar da senha: %q", cmd.Text)
		}
	}
}
