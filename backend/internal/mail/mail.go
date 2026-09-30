// Package mail envia os e-mails transacionais da plataforma — hoje, os da
// conta do usuário (redefinição e confirmação de troca de senha).
package mail

import (
	"context"
	"errors"
	"log/slog"
)

// Message é um e-mail pronto para envio, com versão em texto e em HTML.
type Message struct {
	// To é o destinatário; ToName, opcional, vira o nome de exibição.
	To      string
	ToName  string
	Subject string
	Text    string
	HTML    string
}

// Sender entrega uma mensagem.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// ErrNotConfigured é devolvido pelo LogSender fora de desenvolvimento: o
// e-mail não saiu, e quem chamou precisa registrar isso como falha.
var ErrNotConfigured = errors.New("envio de e-mail não configurado (defina SMTP_HOST)")

// LogSender substitui o SMTP quando SMTP_HOST está vazio.
//
// Em desenvolvimento registra a mensagem inteira no log, para dar para testar
// o fluxo sem servidor de e-mail. Fora dele registra só o assunto: o corpo
// carrega links com token, e token não vai para log.
type LogSender struct {
	log      *slog.Logger
	showBody bool
}

func NewLogSender(log *slog.Logger, showBody bool) *LogSender {
	return &LogSender{log: log.With("component", "mail"), showBody: showBody}
}

func (s *LogSender) Send(ctx context.Context, msg Message) error {
	if s.showBody {
		s.log.WarnContext(ctx, "SMTP não configurado: e-mail NÃO enviado; conteúdo abaixo (só em desenvolvimento)",
			"to", msg.To, "subject", msg.Subject, "text", msg.Text)
		return nil
	}
	return ErrNotConfigured
}
