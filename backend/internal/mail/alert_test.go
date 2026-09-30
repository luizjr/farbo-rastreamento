package mail

import (
	"strings"
	"testing"
)

func TestAlertEmailEscapesAndShowsDetails(t *testing.T) {
	speed, limit, lat, lon := 127.4, 100.0, -23.55052, -46.63331
	msg, err := renderAlert(Alert{
		To: "ana@cliente.test", Name: "Ana Souza", Subject: "Alerta: Excesso de velocidade — x",
		Severity: SeverityWarning, Title: "Excesso de velocidade",
		Summary: "O veículo passou do limite.", Vehicle: `<img src=x onerror=alert(1)>`, Plate: "ABC1D23",
		When: "30/09/2026 às 02:14 (horário de Brasília)", Speed: &speed, Limit: &limit,
		Latitude: &lat, Longitude: &lon, Suppressed: 3, Cooldown: "30 minutos",
		ActionURL: "https://painel.test/veiculos/1", SettingsURL: "https://painel.test/alertas",
		AppURL: "https://painel.test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(msg.HTML, "<img src=x") || !strings.Contains(msg.HTML, "&lt;img src=x") {
		t.Error("nome do veículo precisa sair escapado no HTML")
	}
	for _, want := range []string{"Olá, Ana!", "127 km/h", "100 km/h", "-23.55052, -46.63331",
		"https://www.openstreetmap.org/?mlat=-23.550520&amp;mlon=-46.633310", "mais <strong>3</strong> vezes",
		"30 minutos", "https://painel.test/veiculos/1", "https://painel.test/alertas", "#b45309"} {
		if !strings.Contains(msg.HTML, want) {
			t.Errorf("HTML sem %q", want)
		}
	}
	for _, want := range []string{"Excesso de velocidade", "Velocidade: 127 km/h", "Limite: 100 km/h",
		"Mapa: https://www.openstreetmap.org/", "mais 3 vezes", "Escolha quais alertas receber"} {
		if !strings.Contains(msg.Text, want) {
			t.Errorf("texto sem %q", want)
		}
	}
	if msg.To != "ana@cliente.test" || msg.Subject == "" {
		t.Errorf("destinatário/assunto: %q %q", msg.To, msg.Subject)
	}
}

func TestAlertEmailForCentralHasNoSettingsLink(t *testing.T) {
	msg, err := renderAlert(Alert{To: "central@farbo.test", Severity: SeverityCritical, Title: "SOS",
		Summary: "x", Vehicle: "Carro", When: "agora", ActionURL: "https://painel.test/dashboard", AppURL: "https://painel.test"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(msg.HTML, "/alertas") || strings.Contains(msg.Text, "Escolha quais alertas") {
		t.Error("a central não tem tela de preferências: o e-mail não aponta para ela")
	}
	if !strings.Contains(msg.HTML, "Alerta de segurança") || !strings.Contains(msg.HTML, "#b3261e") {
		t.Error("alerta crítico usa a faixa vermelha de segurança")
	}
	if strings.Contains(msg.HTML, "Velocidade") || strings.Contains(msg.HTML, "repetiu") {
		t.Error("sem velocidade nem repetidos, essas linhas não aparecem")
	}
}
