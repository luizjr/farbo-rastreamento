package config

import (
	_ "embed"
	"fmt"
	"strings"
)

// Segredos da instalação: o que é recusado e em que ambiente.
//
// Ausente ou curto demais é erro em qualquer ambiente. Valor de exemplo,
// padrão óbvio, JWT_SECRET que claramente não foi sorteado e segredo repetido
// entre duas variáveis são erro fora do desenvolvimento; com APP_ENV
// development ou test viram aviso no log (Config.Warnings), para não travar
// quem sobe o projeto na própria máquina com o .env de sempre. O compose sobe
// com APP_ENV=production quando a variável não é definida.

// placeholdersFile é a lista única de valores de exemplo e padrões óbvios.
//
//go:embed placeholders.txt
var placeholdersFile string

var placeholderExact, placeholderParts = parsePlaceholders(placeholdersFile)

// Comandos sugeridos nas mensagens de erro.
const (
	genJWTSecret = "openssl rand -base64 48"
	genPassword  = "openssl rand -base64 32"
)

func parsePlaceholders(text string) (map[string]bool, []string) {
	exact := map[string]bool{}
	var parts []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if part, ok := strings.CutPrefix(line, "*"); ok {
			if n := normalizeSecret(part); n != "" {
				parts = append(parts, n)
			}
			continue
		}
		if n := normalizeSecret(line); n != "" {
			exact[n] = true
		}
	}
	return exact, parts
}

// normalizeSecret deixa só letras minúsculas e dígitos ASCII, para que
// "Troque_Esta_Senha" e "troque-esta-senha" contem como o mesmo valor.
func normalizeSecret(value string) string {
	var b strings.Builder
	for _, c := range []byte(strings.ToLower(value)) {
		if ('a' <= c && c <= 'z') || ('0' <= c && c <= '9') {
			b.WriteByte(c)
		}
	}
	return b.String()
}

// IsPlaceholder diz se o valor é um exemplo versionado, um padrão óbvio ou
// texto de modelo (<gere-um-valor>, ${VAR} não expandida, só pontuação): não
// serve como segredo nem como credencial inicial. Vazio não é placeholder —
// ausência é tratada à parte por quem chama.
func IsPlaceholder(value string) bool {
	v := strings.TrimSpace(value)
	if v == "" {
		return false
	}
	if (strings.HasPrefix(v, "<") && strings.HasSuffix(v, ">")) ||
		strings.HasPrefix(v, "${") || strings.HasPrefix(v, "$(") {
		return true
	}
	n := normalizeSecret(v)
	if n == "" || placeholderExact[n] {
		return true
	}
	for _, part := range placeholderParts {
		if strings.Contains(n, part) {
			return true
		}
	}
	return false
}

// minJWTSecretLen é o mínimo do JWT_SECRET, em bytes: 32 bytes sorteados dão
// 256 bits, o tamanho da chave do HS256.
const minJWTSecretLen = 32

// jwtSecretProblem diz por que o JWT_SECRET não parece ter sido sorteado, ou
// "" se nada óbvio aparece. Não mede entropia: só pega o que denuncia um valor
// escrito à mão (exemplo, poucos caracteres, repetição, sequência). Passar
// aqui não prova nada; o valor ainda precisa vir de um gerador (openssl rand).
//
// Os limites foram escolhidos para não recusar um valor de fato sorteado:
// em 300 mil valores de openssl rand -hex 16 / -base64 48 nenhum cai aqui.
func jwtSecretProblem(secret string) string {
	if IsPlaceholder(secret) {
		return "é um valor de exemplo ou um padrão conhecido"
	}
	b := []byte(secret)
	distinct := map[byte]bool{}
	for _, c := range b {
		distinct[c] = true
	}
	if len(distinct) < 8 {
		return "não parece aleatório (usa poucos caracteres diferentes)"
	}

	// Maior trecho de um caractere repetido (aaaaaaaa) e de sequência
	// (abcdefgh, 87654321); e quantos passos mudam no máximo uma posição.
	sameRun, stepRun, longestSame, longestStep, smallSteps := 1, 1, 1, 1, 0
	for i := 1; i < len(b); i++ {
		d := int(b[i]) - int(b[i-1])
		if d == 0 {
			sameRun++
		} else {
			sameRun = 1
		}
		switch {
		case (d == 1 || d == -1) && i >= 2 && d == int(b[i-1])-int(b[i-2]):
			stepRun++
		case d == 1 || d == -1:
			stepRun = 2
		default:
			stepRun = 1
		}
		if d >= -1 && d <= 1 {
			smallSteps++
		}
		longestSame, longestStep = max(longestSame, sameRun), max(longestStep, stepRun)
	}
	if longestSame >= 8 {
		return "não parece aleatório (tem um caractere repetido em sequência)"
	}
	if longestStep >= 8 {
		return "não parece aleatório (tem uma sequência como abcdefgh ou 12345678)"
	}
	if smallSteps*4 >= (len(b)-1)*3 {
		return "não parece aleatório (é quase todo repetição ou sequência)"
	}
	// Um trecho curto repetido até completar o tamanho (Farbo2024!Farbo2024!...).
	for period := 1; period <= len(b)/2; period++ {
		repeated := true
		for i := period; i < len(b); i++ {
			if b[i] != b[i-period] {
				repeated = false
				break
			}
		}
		if repeated {
			return "não parece aleatório (é um trecho curto repetido)"
		}
	}
	return ""
}

// IsDevelopment diz se o ambiente tolera segredos fracos (só com aviso).
// Qualquer outro valor de APP_ENV — inclusive um digitado errado — é tratado
// como produção.
func (c *Config) IsDevelopment() bool {
	switch strings.ToLower(strings.TrimSpace(c.Env)) {
	case "development", "test":
		return true
	}
	return false
}

// validateSecrets confere JWT_SECRET, POSTGRES_PASSWORD, REDIS_PASSWORD (com
// o Redis ligado), ADMIN_EMAIL e ADMIN_PASSWORD. Roda dentro de Load: a
// recusa acontece antes de conectar no banco, abrir portas ou criar usuário.
// As mensagens nunca incluem o valor.
func (c *Config) validateSecrets() error {
	var fatal, weak []string

	jwt := string(c.Auth.JWTSecret)
	switch {
	case jwt == "":
		fatal = append(fatal, "JWT_SECRET é obrigatório: gere com "+genJWTSecret)
	case len(jwt) < minJWTSecretLen:
		fatal = append(fatal, fmt.Sprintf(
			"JWT_SECRET precisa de ao menos %d caracteres: gere com %s", minJWTSecretLen, genJWTSecret))
	default:
		if problem := jwtSecretProblem(jwt); problem != "" {
			weak = append(weak, "JWT_SECRET "+problem+": gere um valor aleatório com "+genJWTSecret)
		}
	}

	switch {
	case c.Postgres.Password == "":
		fatal = append(fatal, "POSTGRES_PASSWORD é obrigatório: gere com "+genPassword)
	case IsPlaceholder(c.Postgres.Password):
		weak = append(weak, "POSTGRES_PASSWORD é um valor de exemplo ou um padrão conhecido: gere com "+
			genPassword+" e troque também no PostgreSQL (README, \"Trocando os segredos\")")
	}

	if c.Redis.Enabled {
		switch {
		case c.Redis.Password == "":
			weak = append(weak, "REDIS_PASSWORD é obrigatório com REDIS_ENABLED=true: gere com "+genPassword)
		case IsPlaceholder(c.Redis.Password):
			weak = append(weak, "REDIS_PASSWORD é um valor de exemplo ou um padrão conhecido: gere com "+genPassword)
		}
	}

	if IsPlaceholder(c.Bootstrap.AdminPassword) {
		weak = append(weak, "ADMIN_PASSWORD é a senha de exemplo ou um padrão conhecido: escolha uma senha "+
			"própria (ou deixe vazio depois que o primeiro acesso existir)")
	}
	if IsPlaceholder(c.Bootstrap.AdminEmail) {
		weak = append(weak, "ADMIN_EMAIL é o e-mail de exemplo: use o e-mail real de quem administra")
	}

	// Cada segredo precisa ser próprio: com a senha do banco igual ao
	// JWT_SECRET, vazar uma é forjar token de administrador.
	named := []struct{ name, value string }{
		{"JWT_SECRET", jwt},
		{"POSTGRES_PASSWORD", c.Postgres.Password},
		{"REDIS_PASSWORD", c.Redis.Password},
		{"ADMIN_PASSWORD", c.Bootstrap.AdminPassword},
	}
	for i := range named {
		for j := i + 1; j < len(named); j++ {
			if named[i].value != "" && named[i].value == named[j].value {
				weak = append(weak, fmt.Sprintf("%s e %s têm o mesmo valor: cada um precisa ser próprio",
					named[i].name, named[j].name))
			}
		}
	}

	problems := fatal
	if !c.IsDevelopment() {
		problems = append(problems, weak...)
	} else {
		c.Warnings = append(c.Warnings, weak...)
	}
	switch len(problems) {
	case 0:
		return nil
	case 1:
		if len(fatal) == 1 {
			return fmt.Errorf("%s", problems[0])
		}
	}
	msg := "configuração recusada:\n  - " + strings.Join(problems, "\n  - ")
	if len(problems) > len(fatal) {
		msg += fmt.Sprintf("\nCom APP_ENV=%q isso é recusado; só APP_ENV=development ou test tolera, "+
			"com aviso no log.", c.Env)
	}
	return fmt.Errorf("%s", msg)
}
