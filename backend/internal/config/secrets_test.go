package config

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
)

// Valores de exemplo que já estiveram versionados (.env.example, README,
// docker-compose.yml). Repetidos aqui de propósito: se alguém tirar um deles
// de placeholders.txt, o teste quebra.
var versionedPlaceholders = []string{
	"troque-esta-senha",
	"troque-esta-senha-do-redis",
	"troque-esta-senha-do-grafana",
	"troque-por-uma-string-aleatoria-de-no-minimo-32-caracteres",
	"uma-senha-com-10-ou-mais-caracteres",
	"admin",
	"...",
}

func randomSecret(t *testing.T, n int) string {
	t.Helper()
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(buf)
}

// secretEnv define um ambiente de produção válido, com segredos próprios, e
// devolve os valores para o teste trocar um de cada vez.
func secretEnv(t *testing.T) map[string]string {
	t.Helper()
	env := map[string]string{
		"APP_ENV":           "production",
		"JWT_SECRET":        randomSecret(t, 48),
		"POSTGRES_PASSWORD": randomSecret(t, 24),
		"REDIS_ENABLED":     "true",
		"REDIS_PASSWORD":    randomSecret(t, 24),
		"ADMIN_EMAIL":       "operacao@farbo.com.br",
		"ADMIN_PASSWORD":    "Kx9!vT2#pQ7&mW4z",
		// Isola o teste de variáveis do ambiente de quem roda a suíte.
		"APP_URL":      "https://painel.farbo.com.br",
		"CORS_ORIGINS": "https://painel.farbo.com.br",
		"SMTP_HOST":    "",
	}
	for k, v := range env {
		t.Setenv(k, v)
	}
	return env
}

func loadWith(t *testing.T, overrides map[string]string) (*Config, error) {
	t.Helper()
	for k, v := range overrides {
		t.Setenv(k, v)
	}
	return Load()
}

func TestLoadAcceptsUniqueSecrets(t *testing.T) {
	secretEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("instalação com segredos próprios devia subir: %v", err)
	}
	if len(cfg.Warnings) != 0 {
		t.Errorf("nenhum aviso esperado, veio %v", cfg.Warnings)
	}
}

func TestLoadRejectsMissingAndShortSecrets(t *testing.T) {
	for _, appEnv := range []string{"production", "development"} {
		for _, tc := range []struct {
			name, key, value, want string
		}{
			{"JWT ausente", "JWT_SECRET", "", "JWT_SECRET é obrigatório"},
			{"JWT só espaços", "JWT_SECRET", "   ", "JWT_SECRET é obrigatório"},
			{"JWT curto", "JWT_SECRET", strings.Repeat("x", 31), "JWT_SECRET precisa de ao menos 32"},
			{"JWT curto sorteado", "JWT_SECRET", randomSecret(t, 24)[:31], "JWT_SECRET precisa de ao menos 32"},
			{"Postgres ausente", "POSTGRES_PASSWORD", "", "POSTGRES_PASSWORD é obrigatório"},
		} {
			t.Run(appEnv+"/"+tc.name, func(t *testing.T) {
				secretEnv(t)
				_, err := loadWith(t, map[string]string{"APP_ENV": appEnv, tc.key: tc.value})
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("esperava erro com %q, veio %v", tc.want, err)
				}
				if !strings.Contains(err.Error(), "gere com openssl rand") {
					t.Errorf("a mensagem devia dizer como gerar o valor: %v", err)
				}
			})
		}
	}
}

func TestLoadRejectsEveryPlaceholderInProduction(t *testing.T) {
	// Cada valor versionado, com variações de caixa e separador, e cada
	// entrada de placeholders.txt.
	values := append([]string{}, versionedPlaceholders...)
	for _, v := range versionedPlaceholders {
		values = append(values, strings.ToUpper(v), strings.ReplaceAll(v, "-", "_"), " "+v+" ")
	}
	for v := range placeholderExact {
		values = append(values, v)
	}

	for _, key := range []string{"JWT_SECRET", "POSTGRES_PASSWORD", "REDIS_PASSWORD", "ADMIN_PASSWORD"} {
		for _, value := range values {
			secretEnv(t)
			_, err := loadWith(t, map[string]string{key: value})
			if err == nil {
				t.Errorf("%s=%q foi aceito em produção", key, value)
				continue
			}
			if !strings.Contains(err.Error(), key) {
				t.Errorf("%s=%q: o erro devia nomear a variável: %v", key, value, err)
			}
			if trimmed := strings.TrimSpace(value); len(trimmed) >= 12 && strings.Contains(err.Error(), trimmed) {
				t.Errorf("%s: a mensagem de erro repete o valor recusado", key)
			}
		}
	}

	// JWT_SECRET de exemplo com tamanho suficiente: recusado por ser exemplo,
	// não por ser curto.
	secretEnv(t)
	_, err := loadWith(t, map[string]string{"JWT_SECRET": "troque-por-uma-string-aleatoria-de-no-minimo-32-caracteres"})
	if err == nil || !strings.Contains(err.Error(), "JWT_SECRET é um valor de exemplo") {
		t.Fatalf("JWT_SECRET do .env.example devia ser recusado como exemplo: %v", err)
	}
	if !strings.Contains(err.Error(), "openssl rand -base64 48") {
		t.Errorf("a mensagem devia dizer como gerar o segredo: %v", err)
	}

	secretEnv(t)
	if _, err := loadWith(t, map[string]string{"ADMIN_EMAIL": "voce@exemplo.com"}); err == nil ||
		!strings.Contains(err.Error(), "ADMIN_EMAIL") {
		t.Errorf("ADMIN_EMAIL de exemplo devia ser recusado: %v", err)
	}
}

func TestLoadListsAllProblemsOfTheCopiedExample(t *testing.T) {
	// Exatamente o .env.example antigo copiado sem trocar nada.
	secretEnv(t)
	_, err := loadWith(t, map[string]string{
		"POSTGRES_PASSWORD": "troque-esta-senha",
		"REDIS_PASSWORD":    "troque-esta-senha-do-redis",
		"JWT_SECRET":        "troque-por-uma-string-aleatoria-de-no-minimo-32-caracteres",
		"ADMIN_EMAIL":       "voce@exemplo.com",
		"ADMIN_PASSWORD":    "uma-senha-com-10-ou-mais-caracteres",
	})
	if err == nil {
		t.Fatal("o .env.example antigo foi aceito em produção")
	}
	for _, key := range []string{"JWT_SECRET", "POSTGRES_PASSWORD", "REDIS_PASSWORD", "ADMIN_EMAIL", "ADMIN_PASSWORD"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("o erro devia apontar %s: %v", key, err)
		}
	}
}

func TestLoadRedisPasswordOnlyWhenEnabled(t *testing.T) {
	secretEnv(t)
	if _, err := loadWith(t, map[string]string{"REDIS_ENABLED": "false", "REDIS_PASSWORD": ""}); err != nil {
		t.Errorf("sem Redis a senha dele não importa: %v", err)
	}
	secretEnv(t)
	if _, err := loadWith(t, map[string]string{"REDIS_PASSWORD": ""}); err == nil ||
		!strings.Contains(err.Error(), "REDIS_PASSWORD é obrigatório") {
		t.Errorf("Redis ligado sem senha devia ser recusado em produção: %v", err)
	}
}

func TestLoadRejectsReusedSecrets(t *testing.T) {
	env := secretEnv(t)
	_, err := loadWith(t, map[string]string{"POSTGRES_PASSWORD": env["JWT_SECRET"]})
	if err == nil || !strings.Contains(err.Error(), "JWT_SECRET e POSTGRES_PASSWORD têm o mesmo valor") {
		t.Fatalf("segredo repetido devia ser recusado: %v", err)
	}
}

func TestLoadDevelopmentOnlyWarns(t *testing.T) {
	secretEnv(t)
	cfg, err := loadWith(t, map[string]string{
		"APP_ENV":           "development",
		"JWT_SECRET":        "troque-por-uma-string-aleatoria-de-no-minimo-32-caracteres",
		"POSTGRES_PASSWORD": "troque-esta-senha",
		"ADMIN_PASSWORD":    "uma-senha-com-10-ou-mais-caracteres",
	})
	if err != nil {
		t.Fatalf("desenvolvimento tolera os exemplos (com aviso): %v", err)
	}
	joined := strings.Join(cfg.Warnings, "\n")
	for _, key := range []string{"JWT_SECRET", "POSTGRES_PASSWORD", "ADMIN_PASSWORD"} {
		if !strings.Contains(joined, key) {
			t.Errorf("faltou o aviso de %s: %v", key, cfg.Warnings)
		}
	}
}

func TestUnknownAppEnvIsStrict(t *testing.T) {
	for _, appEnv := range []string{"production", "prod", "staging", "producao", "Development-ish"} {
		secretEnv(t)
		_, err := loadWith(t, map[string]string{"APP_ENV": appEnv, "POSTGRES_PASSWORD": "troque-esta-senha"})
		if err == nil {
			t.Errorf("APP_ENV=%q tolerou senha de exemplo", appEnv)
		}
	}
	for _, appEnv := range []string{"development", "test", "Development"} {
		secretEnv(t)
		if _, err := loadWith(t, map[string]string{"APP_ENV": appEnv, "POSTGRES_PASSWORD": "troque-esta-senha"}); err != nil {
			t.Errorf("APP_ENV=%q devia só avisar: %v", appEnv, err)
		}
	}
}

func TestIsPlaceholder(t *testing.T) {
	for _, v := range []string{
		"troque-esta-senha", "Troque_Esta_Senha", "  TROQUE-ESTA-SENHA  ", "troque-esta-senha-2",
		"voce@exemplo.com", "admin@example.com", "<gere-com-openssl>", "${JWT_SECRET}", "$(openssl rand -hex 32)",
		"...", "---", "admin", "Admin", "changeme", "CHANGE_ME_PLEASE", "my-placeholder-value", "secret",
	} {
		if !IsPlaceholder(v) {
			t.Errorf("IsPlaceholder(%q) = false", v)
		}
	}
	for _, v := range []string{
		"", "operacao@farbo.com.br", "Kx9!vT2#pQ7&mW4z", "senha-forte-da-central-2026!", "segredo-da-frota-sul",
		randomSecret(t, 32),
	} {
		if IsPlaceholder(v) {
			t.Errorf("IsPlaceholder(%q) = true", v)
		}
	}
}

func TestJWTSecretProblem(t *testing.T) {
	for _, weak := range []string{
		strings.Repeat("a", 40),
		strings.Repeat("ab", 20),
		"abcdefghijklmnopqrstuvwxyz0123456789",
		"12345678901234567890123456789012",
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaa1234567",
		"aabbccddeeffgghhiijjkkllmmnnooppqq",
		"Farbo2024!Farbo2024!Farbo2024!Farbo2024!",
		"troque-por-uma-string-aleatoria-de-no-minimo-32-caracteres",
	} {
		if jwtSecretProblem(weak) == "" {
			t.Errorf("jwtSecretProblem(%q) não viu problema", weak)
		}
	}
	// Valores sorteados de verdade nunca podem ser recusados.
	for i := 0; i < 2000; i++ {
		buf := make([]byte, 48)
		if _, err := rand.Read(buf); err != nil {
			t.Fatal(err)
		}
		for _, s := range []string{
			base64.StdEncoding.EncodeToString(buf),
			base64.RawURLEncoding.EncodeToString(buf[:24]),
			hex.EncodeToString(buf[:16]),
			hex.EncodeToString(buf[:32]),
		} {
			if problem := jwtSecretProblem(s); problem != "" {
				t.Fatalf("segredo sorteado recusado (%s): %q", problem, s)
			}
		}
	}
}
