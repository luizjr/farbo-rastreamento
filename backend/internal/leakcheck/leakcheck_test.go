package leakcheck

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// secret muda de forma em cada codificação: aspas, barra, sinais de HTML,
// caracteres de URL e um não-ASCII.
const secret = `Ap"<n>&\é'9+/=`

// O detector precisa achar o segredo em cada disfarce — senão um teste
// "sem vazamento" não prova nada.
func TestFindCatchesEveryForm(t *testing.T) {
	var escaped strings.Builder
	for _, b := range []byte(secret) {
		fmt.Fprintf(&escaped, "\\x%02x", b)
	}
	jsonEscaped, _ := json.Marshal(map[string]string{"payload": "cmd," + secret + "#"})

	cases := map[string][]byte{
		"json":             jsonEscaped,
		"hex maiúsculo":    []byte(`{"payload":"78780F` + strings.ToUpper(hex.EncodeToString([]byte(secret))) + `"}`),
		"hex minúsculo":    []byte(`{"payload":"` + hex.EncodeToString([]byte(secret)) + `"}`),
		`\xNN`:             mustJSON(map[string]string{"payload": "xx" + escaped.String()}),
		"base64":           mustJSON([]string{base64.StdEncoding.EncodeToString([]byte(secret))}),
		"chave":            mustJSON(map[string]int{secret: 1}),
		"caixa alta":       mustJSON(map[string]any{"a": []any{map[string]string{"b": strings.ToUpper(secret)}}}),
		"json dentro json": mustJSON(map[string]string{"data": string(jsonEscaped)}),
		"texto não-JSON":   []byte("resposta: " + secret),
	}
	for name, body := range cases {
		if len(Find(body, secret)) == 0 {
			t.Errorf("%s: segredo não detectado em %s", name, body)
		}
	}
}

func TestFindIgnoresCleanBody(t *testing.T) {
	body := mustJSON(map[string]any{"payload": "xx\\x0FDYD,***#", "apnPasswordSet": true})
	if found := Find(body, secret, "Zq7Kx2Wm9Rt4"); len(found) > 0 {
		t.Fatalf("falso positivo: %v", found)
	}
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
