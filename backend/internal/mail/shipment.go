package mail

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strings"
)

// TrackingURL é a página pública de rastreio de um código.
func TrackingURL(code string) string {
	return "https://www.melhorrastreio.com.br/rastreio/" + url.PathEscape(code)
}

// ShipmentMailer avisa o cliente nos marcos da entrega do rastreador.
type ShipmentMailer struct {
	sender Sender
	appURL string
}

func NewShipmentMailer(sender Sender, appURL string) *ShipmentMailer {
	return &ShipmentMailer{sender: sender, appURL: strings.TrimRight(appURL, "/")}
}

// Shipment é o que os e-mails mostram.
type Shipment struct {
	To           string
	Name         string
	Vehicle      string
	Carrier      string
	TrackingCode string
}

type shipmentData struct {
	Name         string
	AppURL       string
	ActionURL    string
	Vehicle      string
	Carrier      string
	TrackingCode string
	TrackingURL  string
}

func (m *ShipmentMailer) data(s Shipment) shipmentData {
	d := shipmentData{
		Name: firstName(s.Name), AppURL: m.appURL, ActionURL: m.appURL + "/meus-veiculos",
		Vehicle: s.Vehicle, Carrier: s.Carrier, TrackingCode: s.TrackingCode,
	}
	if s.TrackingCode != "" {
		d.TrackingURL = TrackingURL(s.TrackingCode)
	}
	return d
}

// Shipped: o rastreador saiu, com o código para acompanhar.
func (m *ShipmentMailer) Shipped(ctx context.Context, s Shipment) error {
	msg, err := renderShipment(shippedTemplates, m.data(s))
	if err != nil {
		return err
	}
	msg.To, msg.ToName = s.To, s.Name
	msg.Subject = "Seu rastreador foi enviado — Farbo Rastreadores"
	return m.sender.Send(ctx, msg)
}

// Delivered: o rastreador chegou; hora de agendar a instalação.
func (m *ShipmentMailer) Delivered(ctx context.Context, s Shipment) error {
	msg, err := renderShipment(deliveredTemplates, m.data(s))
	if err != nil {
		return err
	}
	msg.To, msg.ToName = s.To, s.Name
	msg.Subject = "Seu rastreador chegou — agende a instalação"
	return m.sender.Send(ctx, msg)
}

func renderShipment(t templatePair, data shipmentData) (Message, error) {
	var text, html bytes.Buffer
	if err := t.text.Execute(&text, data); err != nil {
		return Message{}, fmt.Errorf("montando e-mail (texto): %w", err)
	}
	if err := t.html.ExecuteTemplate(&html, "layout", data); err != nil {
		return Message{}, fmt.Errorf("montando e-mail (HTML): %w", err)
	}
	return Message{Text: text.String(), HTML: html.String()}, nil
}

const shippedHTML = `{{define "content"}}
<p style="margin:0 0 8px;font-size:12px;font-weight:800;letter-spacing:3px;text-transform:uppercase;color:#15803d;">Seu pedido</p>
<h1 style="margin:0 0 16px;font-size:22px;line-height:1.3;color:#0d130e;">Seu rastreador foi enviado</h1>
<p style="margin:0 0 12px;font-size:15px;line-height:1.6;color:#334155;">Olá{{with .Name}}, {{.}}{{end}}!</p>
<p style="margin:0 0 12px;font-size:15px;line-height:1.6;color:#334155;">O rastreador do <strong>{{.Vehicle}}</strong> já está configurado, com o chip M2M, e saiu para entrega{{with .Carrier}} por <strong>{{.}}</strong>{{end}}.</p>
{{if .TrackingCode}}<p style="margin:0 0 4px;font-size:13px;color:#475569;">Código de rastreio</p>
<p style="margin:0 0 4px;font-size:20px;font-weight:800;letter-spacing:1px;color:#0d130e;">{{.TrackingCode}}</p>
{{template "button" (button .TrackingURL "Acompanhar a entrega")}}{{else}}
{{template "button" (button .ActionURL "Acompanhar no painel")}}{{end}}
<p style="margin:0;font-size:14px;line-height:1.6;color:#475569;">Você também acompanha cada etapa no painel, em Meus veículos. Quando chegar, é só agendar a instalação com um dos prestadores recomendados.</p>
{{end}}`

const shippedText = `Olá{{with .Name}}, {{.}}{{end}}!

O rastreador do {{.Vehicle}} já está configurado, com o chip M2M, e saiu para entrega{{with .Carrier}} por {{.}}{{end}}.
{{if .TrackingCode}}
Código de rastreio: {{.TrackingCode}}
Acompanhe a entrega: {{.TrackingURL}}
{{end}}
Você também acompanha cada etapa no painel, em Meus veículos:
{{.ActionURL}}

— Farbo Rastreadores
`

const deliveredHTML = `{{define "content"}}
<p style="margin:0 0 8px;font-size:12px;font-weight:800;letter-spacing:3px;text-transform:uppercase;color:#15803d;">Seu pedido</p>
<h1 style="margin:0 0 16px;font-size:22px;line-height:1.3;color:#0d130e;">Seu rastreador chegou</h1>
<p style="margin:0 0 12px;font-size:15px;line-height:1.6;color:#334155;">Olá{{with .Name}}, {{.}}{{end}}!</p>
<p style="margin:0;font-size:15px;line-height:1.6;color:#334155;">O rastreador do <strong>{{.Vehicle}}</strong> foi entregue. Agora é só agendar a instalação com um dos nossos prestadores recomendados — a instalação é combinada e paga direto com eles.</p>
{{template "button" (button .ActionURL "Ver instaladores")}}
<p style="margin:0;font-size:14px;line-height:1.6;color:#475569;">Depois de instalado, o veículo aparece no mapa do painel assim que o rastreador der o primeiro sinal.</p>
{{end}}`

const deliveredText = `Olá{{with .Name}}, {{.}}{{end}}!

O rastreador do {{.Vehicle}} foi entregue. Agora é só agendar a instalação com um dos nossos prestadores recomendados — a instalação é combinada e paga direto com eles.

Veja os instaladores no painel, em Meus veículos:
{{.ActionURL}}

Depois de instalado, o veículo aparece no mapa do painel assim que o rastreador der o primeiro sinal.

— Farbo Rastreadores
`

var (
	shippedTemplates   = mustTemplates(shippedText, shippedHTML)
	deliveredTemplates = mustTemplates(deliveredText, deliveredHTML)
)
