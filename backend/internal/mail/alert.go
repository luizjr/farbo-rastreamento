package mail

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
)

// AlertMailer envia os alertas dos veículos (internal/alerts).
type AlertMailer struct {
	sender Sender
}

func NewAlertMailer(sender Sender) *AlertMailer { return &AlertMailer{sender: sender} }

// Severidade do alerta: muda a cor da faixa do e-mail.
const (
	SeverityCritical = "critical"
	SeverityWarning  = "warning"
	SeverityInfo     = "info"
)

// Alert é o que o e-mail de alerta mostra. As frases já vêm prontas do
// pacote alerts; aqui só se monta a mensagem.
type Alert struct {
	To       string
	Name     string
	Subject  string
	Severity string
	// Title é a manchete ("Ignição ligada no horário de vigilância").
	Title string
	// Summary é a frase que explica o que aconteceu.
	Summary string
	Vehicle string
	Plate   string
	// When é a data e hora já formatadas no fuso da central.
	When string
	// Speed e Limit em km/h; nil quando não se aplicam.
	Speed *float64
	Limit *float64
	// Latitude/Longitude do evento, quando o rastreador mandou posição.
	Latitude  *float64
	Longitude *float64
	Address   string
	// Suppressed: ocorrências iguais seguradas desde o último e-mail.
	Suppressed int
	// Cooldown por extenso ("30 minutos"): explica o resumo dos repetidos.
	Cooldown    string
	ActionURL   string
	SettingsURL string
	AppURL      string
}

type alertData struct {
	Alert
	FirstName   string
	Accent      string
	Label       string
	SpeedText   string
	LimitText   string
	MapURL      string
	Coordinates string
}

func (m *AlertMailer) Alert(ctx context.Context, a Alert) error {
	msg, err := renderAlert(a)
	if err != nil {
		return err
	}
	return m.sender.Send(ctx, msg)
}

func renderAlert(a Alert) (Message, error) {
	d := alertData{Alert: a, FirstName: firstName(a.Name)}
	switch a.Severity {
	case SeverityCritical:
		d.Accent, d.Label = "#b3261e", "Alerta de segurança"
	case SeverityWarning:
		d.Accent, d.Label = "#b45309", "Alerta"
	default:
		d.Accent, d.Label = "#15803d", "Aviso"
	}
	if a.Speed != nil {
		d.SpeedText = strconv.FormatFloat(*a.Speed, 'f', 0, 64) + " km/h"
	}
	if a.Limit != nil {
		d.LimitText = strconv.FormatFloat(*a.Limit, 'f', 0, 64) + " km/h"
	}
	if a.Latitude != nil && a.Longitude != nil {
		lat, lon := *a.Latitude, *a.Longitude
		d.Coordinates = fmt.Sprintf("%.5f, %.5f", lat, lon)
		d.MapURL = fmt.Sprintf("https://www.openstreetmap.org/?mlat=%.6f&mlon=%.6f#map=17/%.6f/%.6f", lat, lon, lat, lon)
	}

	var text, html bytes.Buffer
	if err := alertTemplates.text.Execute(&text, d); err != nil {
		return Message{}, fmt.Errorf("montando e-mail de alerta (texto): %w", err)
	}
	if err := alertTemplates.html.ExecuteTemplate(&html, "layout", d); err != nil {
		return Message{}, fmt.Errorf("montando e-mail de alerta (HTML): %w", err)
	}
	return Message{To: a.To, ToName: a.Name, Subject: a.Subject, Text: text.String(), HTML: html.String()}, nil
}

const alertHTML = `{{define "content"}}
<p style="margin:0 0 8px;font-size:12px;font-weight:800;letter-spacing:3px;text-transform:uppercase;color:{{.Accent}};">{{.Label}}</p>
<h1 style="margin:0 0 16px;font-size:22px;line-height:1.3;color:#0d130e;">{{.Title}}</h1>
<p style="margin:0 0 12px;font-size:15px;line-height:1.6;color:#334155;">Olá{{with .FirstName}}, {{.}}{{end}}!</p>
<p style="margin:0 0 20px;font-size:15px;line-height:1.6;color:#334155;">{{.Summary}}</p>
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="border-left:4px solid {{.Accent}};background:#f8faf9;border-radius:8px;">
<tr><td style="padding:16px 20px;font-size:14px;line-height:1.8;color:#334155;">
<strong>Veículo:</strong> {{.Vehicle}}{{with .Plate}} ({{.}}){{end}}<br>
<strong>Quando:</strong> {{.When}}
{{- with .SpeedText}}<br><strong>Velocidade:</strong> {{.}}{{end}}
{{- with .LimitText}}<br><strong>Limite:</strong> {{.}}{{end}}
{{- if .Address}}<br><strong>Onde:</strong> {{.Address}}{{else if .Coordinates}}<br><strong>Onde:</strong> {{.Coordinates}}{{end}}
</td></tr>
</table>
{{if .Suppressed}}<p style="margin:16px 0 0;font-size:13px;line-height:1.6;color:#475569;">Este alerta se repetiu mais <strong>{{.Suppressed}}</strong> {{if eq .Suppressed 1}}vez{{else}}vezes{{end}} desde o último e-mail. Para não lotar sua caixa, o mesmo alerta do mesmo veículo sai no máximo uma vez a cada {{.Cooldown}}.</p>{{end}}
{{template "button" (button .ActionURL "Ver no painel")}}
{{if .MapURL}}<p style="margin:0 0 20px;font-size:14px;line-height:1.6;color:#475569;"><a href="{{.MapURL}}" style="color:#15803d;">Abrir o local no mapa</a></p>{{end}}
{{if .SettingsURL}}<p style="margin:0;font-size:12px;line-height:1.6;color:#64748b;">Você escolhe quais alertas recebe e o horário de vigilância em <a href="{{.SettingsURL}}" style="color:#15803d;">Alertas</a>, no painel.</p>{{end}}
{{end}}`

const alertText = `Olá{{with .FirstName}}, {{.}}{{end}}!

{{.Title}}
{{.Summary}}

Veículo: {{.Vehicle}}{{with .Plate}} ({{.}}){{end}}
Quando: {{.When}}
{{- with .SpeedText}}
Velocidade: {{.}}{{end}}
{{- with .LimitText}}
Limite: {{.}}{{end}}
{{- if .Address}}
Onde: {{.Address}}{{else if .Coordinates}}
Onde: {{.Coordinates}}{{end}}
{{- with .MapURL}}
Mapa: {{.}}{{end}}
{{if .Suppressed}}
Este alerta se repetiu mais {{.Suppressed}} {{if eq .Suppressed 1}}vez{{else}}vezes{{end}} desde o último e-mail. O mesmo alerta do mesmo veículo sai no máximo uma vez a cada {{.Cooldown}}.
{{end}}
Ver no painel: {{.ActionURL}}
{{with .SettingsURL}}
Escolha quais alertas receber em: {{.}}
{{end}}
— Farbo Rastreadores
`

var alertTemplates = mustTemplates(alertText, alertHTML)
