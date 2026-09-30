package config

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Contrato dos arquivos de implantação na raiz do repositório: o exemplo não
// traz segredo e o compose não tem valor padrão para nenhum. (Estes testes
// leem ../../../; fora do repositório completo eles são pulados.)

var secretName = regexp.MustCompile(`(_PASSWORD|_SECRET|_TOKEN|_API_KEY)$`)

// Os que o compose exige: sem eles não sobe nada.
var requiredSecrets = []string{"POSTGRES_PASSWORD", "REDIS_PASSWORD", "JWT_SECRET", "GRAFANA_PASSWORD"}

func repoFile(t *testing.T, name string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "..", name))
	if os.IsNotExist(err) {
		t.Skipf("%s fora do alcance (teste rodando fora do repositório)", name)
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func TestEnvExampleShipsNoSecretValues(t *testing.T) {
	values := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(repoFile(t, ".env.example")))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, _ := strings.Cut(line, "=")
		values[key] = strings.Trim(strings.TrimSpace(value), `"'`)
	}
	checked := 0
	for key, value := range values {
		if !secretName.MatchString(key) && key != "ADMIN_EMAIL" {
			continue
		}
		checked++
		if value != "" {
			t.Errorf(".env.example traz valor em %s: segredos vêm vazios, para cada instalação gerar o seu", key)
		}
	}
	for _, key := range append(requiredSecrets, "ADMIN_PASSWORD", "ADMIN_EMAIL") {
		if _, ok := values[key]; !ok {
			t.Errorf(".env.example devia listar %s (vazio, com o comando para gerar)", key)
		}
	}
	if checked < 8 {
		t.Errorf("poucos segredos conferidos (%d): o formato do .env.example mudou?", checked)
	}
	if values["APP_ENV"] != "production" {
		t.Errorf("o .env.example deve seguir com APP_ENV=production (validação estrita), veio %q", values["APP_ENV"])
	}
}

// ${VAR}, ${VAR:-padrão}, ${VAR-padrão}, ${VAR:?erro}, ${VAR?erro}
var interpolation = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(?:(:?[-?+])([^}]*))?\}`)

func TestComposeHasNoSecretFallback(t *testing.T) {
	compose := repoFile(t, "docker-compose.yml")
	seen := map[string]bool{}
	for _, m := range interpolation.FindAllStringSubmatchIndex(compose, -1) {
		if m[0] > 0 && compose[m[0]-1] == '$' {
			continue // $${...} é literal no compose
		}
		name := compose[m[2]:m[3]]
		op, def := "", ""
		if m[4] >= 0 {
			op, def = compose[m[4]:m[5]], compose[m[6]:m[7]]
		}
		if !secretName.MatchString(name) {
			continue
		}
		if (op == "-" || op == ":-" || op == "+" || op == ":+") && def != "" {
			t.Errorf("docker-compose.yml dá valor padrão a %s (%s…): segredo não pode ter fallback", name, op)
		}
		for _, required := range requiredSecrets {
			if name == required {
				seen[name] = true
				if op != ":?" {
					t.Errorf("docker-compose.yml usa %s sem ':?': o compose tem de falhar com ela ausente ou vazia", name)
				}
			}
		}
	}
	for _, name := range requiredSecrets {
		if !seen[name] {
			t.Errorf("docker-compose.yml não usa %s", name)
		}
	}
}

func TestComposeKeepsAdminPortsOnLoopback(t *testing.T) {
	compose := repoFile(t, "docker-compose.yml")
	// Blocos de serviço: duas espaços de recuo sob "services:".
	service := regexp.MustCompile(`(?m)^  ([a-z0-9_-]+):\n((?:(?:    .*)?\n)*)`)
	ports := regexp.MustCompile(`(?m)^      - '([^']+)'`)
	found := map[string]bool{}
	for _, block := range service.FindAllStringSubmatch(compose, -1) {
		name, body := block[1], block[2]
		switch name {
		case "grafana", "postgres", "redis", "prometheus":
		default:
			continue
		}
		_, after, ok := strings.Cut(body, "    ports:\n")
		if !ok {
			continue
		}
		for _, p := range ports.FindAllStringSubmatch(after, -1) {
			found[name] = true
			if !strings.HasPrefix(p[1], "127.0.0.1:") {
				t.Errorf("%s publica %q fora do 127.0.0.1", name, p[1])
			}
		}
	}
	if !found["grafana"] {
		t.Error("não achei a porta do grafana no docker-compose.yml")
	}
}
