// Package leakcheck procura credenciais em respostas da API e em eventos do
// WebSocket. Só os testes usam: é o que prova que uma senha sintética não
// sai do backend em forma nenhuma (issue #1).
//
// A senha pode escapar disfarçada: dentro de um pacote binário mostrado em
// hexadecimal, como \xNN, em base64, escapada pelo JSON (\" \\ <) ou
// numa URL. Find procura todas essas formas no texto cru da resposta e em
// cada string do JSON decodificado, recursivamente (chaves e valores), sem
// diferenciar maiúsculas.
package leakcheck

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// Variant é uma forma de escrever o segredo.
type Variant struct {
	Name  string
	Value string
}

// Variants lista as formas em que o segredo pode aparecer num texto.
func Variants(secret string) []Variant {
	raw := []byte(secret)
	var escaped strings.Builder
	for _, b := range raw {
		fmt.Fprintf(&escaped, "\\x%02X", b)
	}
	var unicode strings.Builder
	for _, r := range secret {
		fmt.Fprintf(&unicode, "\\u%04x", r)
	}

	out := []Variant{
		{"texto", secret},
		{"hex", hex.EncodeToString(raw)},
		{`\xNN`, escaped.String()},
		{`\uNNNN`, unicode.String()},
		{"base64", base64.StdEncoding.EncodeToString(raw)},
		{"base64 sem padding", base64.RawStdEncoding.EncodeToString(raw)},
		{"base64url", base64.URLEncoding.EncodeToString(raw)},
		{"base64url sem padding", base64.RawURLEncoding.EncodeToString(raw)},
		{"url (query)", url.QueryEscape(secret)},
		{"url (path)", url.PathEscape(secret)},
	}
	for _, escapeHTML := range []bool{true, false} {
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(escapeHTML)
		_ = enc.Encode(secret)
		quoted := strings.TrimSpace(buf.String())
		out = append(out, Variant{fmt.Sprintf("json (escapeHTML=%v)", escapeHTML),
			strings.TrimSuffix(strings.TrimPrefix(quoted, `"`), `"`)})
		// A string JSON reescapada (JSON dentro de JSON, como o "data" de
		// um evento guardado em texto).
		twice, _ := json.Marshal(quoted)
		out = append(out, Variant{fmt.Sprintf("json duplo (escapeHTML=%v)", escapeHTML),
			strings.TrimSuffix(strings.TrimPrefix(string(twice), `"`), `"`)})
	}

	// Sem repetição e sem formas curtas demais para significar algo.
	seen := map[string]bool{}
	unique := out[:0]
	for _, v := range out {
		key := strings.ToLower(v.Value)
		if len(v.Value) < 4 || seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, v)
	}
	return unique
}

// Find devolve uma descrição de cada lugar em que algum segredo aparece.
// Vazio quer dizer que nada vazou.
func Find(body []byte, secrets ...string) []string {
	var found []string
	check := func(where, text string) {
		lowered := strings.ToLower(text)
		for i, secret := range secrets {
			for _, v := range Variants(secret) {
				if strings.Contains(lowered, strings.ToLower(v.Value)) {
					found = append(found, fmt.Sprintf("segredo #%d (%s) em %s", i+1, v.Name, where))
				}
			}
		}
	}

	check("texto cru", string(body))

	var decoded any
	if json.Unmarshal(body, &decoded) == nil {
		walk("$", decoded, check)
	}
	sort.Strings(found)
	return found
}

func walk(path string, value any, check func(where, text string)) {
	switch v := value.(type) {
	case string:
		check(path, v)
	case []any:
		for i, item := range v {
			walk(fmt.Sprintf("%s[%d]", path, i), item, check)
		}
	case map[string]any:
		for key, item := range v {
			check(path+" (chave)", key)
			walk(path+"."+key, item, check)
		}
	}
}
