package api

import (
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestClientIP(t *testing.T) {
	trustedProxies = []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("172.16.0.0/12")}
	defer func() { trustedProxies = nil }()

	cases := []struct {
		name, remote, xff, realIP, want string
	}{
		{"direto, sem cabeçalho", "203.0.113.7:5000", "", "", "203.0.113.7"},
		{"direto e mentindo no X-Forwarded-For: ignorado", "203.0.113.7:5000", "1.2.3.4", "", "203.0.113.7"},
		{"direto e mentindo no X-Real-IP: ignorado", "203.0.113.7:5000", "", "1.2.3.4", "203.0.113.7"},
		{"pelo nginx (rede do Docker)", "172.18.0.5:40000", "198.51.100.9", "", "198.51.100.9"},
		{"pelo nginx, cliente forjou à esquerda: vale o que o nginx anexou",
			"172.18.0.5:40000", "1.2.3.4, 198.51.100.9", "", "198.51.100.9"},
		{"dois proxies confiáveis na cadeia", "127.0.0.1:1", "198.51.100.9, 172.18.0.9", "", "198.51.100.9"},
		{"lixo no cabeçalho não passa por IP", "172.18.0.5:1", "<script>, 198.51.100.9", "", "198.51.100.9"},
		{"lixo à direita: não confia no resto", "172.18.0.5:1", "198.51.100.9, lixo", "", "172.18.0.5"},
		{"proxy confiável com X-Real-IP", "172.18.0.5:1", "", "198.51.100.10", "198.51.100.10"},
		{"IPv6 direto", "[2001:db8::1]:443", "1.2.3.4", "", "2001:db8::1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = tc.remote
			if tc.xff != "" {
				r.Header.Set("X-Forwarded-For", tc.xff)
			}
			if tc.realIP != "" {
				r.Header.Set("X-Real-IP", tc.realIP)
			}
			if got := clientIP(r); got != tc.want {
				t.Fatalf("clientIP = %q, quer %q", got, tc.want)
			}
		})
	}
}
