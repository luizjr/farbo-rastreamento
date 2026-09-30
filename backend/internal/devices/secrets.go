package devices

import (
	"bytes"
	"strings"
)

// Credenciais do rastreador (§27): a senha APN do chip e a senha de comando
// do aparelho são só de escrita. Nenhuma leitura da API as devolve — nem ao
// admin — e nenhum texto que sai do backend as carrega: histórico de
// comandos, resposta do aparelho, auditoria, eventos do WebSocket e comandos
// de provisionamento passam por RedactBytes/RedactText antes.
//
// A redação é pelo valor, não pela posição no comando: pega a senha também
// num override, num comando livre ou no eco que alguns firmwares devolvem.
// O custo é a redação em excesso quando a senha é curta ou coincide com
// outro trecho (um "1234" dentro do IMEI vira "***") — preferimos isso a
// deixar sair um caractere da senha.

// Redacted é o que aparece no lugar de uma credencial.
const Redacted = "***"

// Secrets devolve as credenciais preenchidas do aparelho.
func (d *Device) Secrets() []string {
	if d == nil {
		return nil
	}
	out := make([]string, 0, 2)
	for _, secret := range []string{d.CommandPassword, d.APNPassword} {
		if secret != "" {
			out = append(out, secret)
		}
	}
	return out
}

// RedactBytes troca cada ocorrência das credenciais nos bytes crus (o pacote
// como vai para o aparelho) por Redacted. Rodar antes de tornar o pacote
// legível garante que a senha não sobra nem como texto nem como \xNN. A
// comparação ignora maiúsculas/minúsculas (ASCII): o eco de um firmware que
// devolve o comando em caixa alta também é pego.
func RedactBytes(payload []byte, secrets []string) []byte {
	out := payload
	for _, secret := range secrets {
		out = replaceFold(out, []byte(secret))
	}
	return out
}

// RedactText é RedactBytes para um texto já legível (resposta do aparelho,
// override cadastrado, comando de provisionamento).
func RedactText(text string, secrets []string) string {
	if text == "" || len(secrets) == 0 {
		return text
	}
	return string(RedactBytes([]byte(text), secrets))
}

// replaceFold substitui needle por Redacted em haystack, sem diferenciar
// caixa ASCII. Devolve haystack intacto (sem cópia) quando não há ocorrência.
func replaceFold(haystack, needle []byte) []byte {
	if len(needle) == 0 || len(haystack) < len(needle) {
		return haystack
	}
	lowered := asciiLower(haystack)
	target := asciiLower(needle)
	if !bytes.Contains(lowered, target) {
		return haystack
	}

	var out bytes.Buffer
	out.Grow(len(haystack))
	for {
		i := bytes.Index(lowered, target)
		if i < 0 {
			out.Write(haystack)
			return out.Bytes()
		}
		out.Write(haystack[:i])
		out.WriteString(Redacted)
		haystack = haystack[i+len(needle):]
		lowered = lowered[i+len(target):]
	}
}

// asciiLower baixa só A-Z, preservando o tamanho em bytes (strings.ToLower
// pode mudar o tamanho de runas Unicode e desalinhar os índices).
func asciiLower(b []byte) []byte {
	out := make([]byte, len(b))
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		out[i] = c
	}
	return out
}

// hasRedaction diz se o texto traz o marcador de credencial redigida.
func hasRedaction(text string) bool { return strings.Contains(text, Redacted) }
