#!/bin/sh
# Entrada do contêiner do Grafana, antes do /run.sh da imagem oficial
# (grafana/grafana:12.4.12, Alpine: /bin/sh é o busybox).
#
# 1. Recusa subir sem senha de administrador própria: vazia, curta, "admin"
#    (o padrão do Grafana) ou qualquer valor da lista de exemplos do backend,
#    backend/internal/config/placeholders.txt, montada no contêiner. A mesma
#    comparação do backend: sem maiúsculas e só letras e dígitos.
# 2. Grava essa senha no banco do Grafana. Ele só lê GF_SECURITY_ADMIN_PASSWORD
#    ao criar o admin, na primeira subida de um volume; depois disso trocar a
#    variável não muda nada, e a senha antiga continuaria valendo. O
#    "grafana cli admin reset-admin-password" aplica a senha no admin (id 1)
#    antes de o servidor subir. Num volume novo ele cria o banco e o admin, e
#    grava a mesma senha.
# 3. Segue para o /run.sh da imagem, que sobe o servidor.
#
# O healthcheck (healthcheck.sh) confirma depois que essa senha entra.
set -eu

PLACEHOLDERS=${FARBO_PLACEHOLDERS:-/etc/farbo/placeholders.txt}
GRAFANA_RUN=${FARBO_GRAFANA_RUN:-/run.sh}
MIN_LENGTH=12

fail() {
	echo "grafana: $*" >&2
	exit 1
}

normalize() {
	printf '%s' "$1" | tr 'A-Z' 'a-z' | tr -cd 'a-z0-9'
}

# Mesma regra de config.IsPlaceholder no backend.
is_placeholder() {
	case $1 in
	'<'*'>' | '${'* | '$('*) return 0 ;;
	esac
	value=$(normalize "$1")
	[ -n "$value" ] || return 0
	while IFS= read -r line || [ -n "$line" ]; do
		case $line in
		'' | '#'*) continue ;;
		'*'*)
			part=$(normalize "${line#\*}")
			[ -n "$part" ] || continue
			case $value in *"$part"*) return 0 ;; esac
			;;
		*)
			[ "$value" = "$(normalize "$line")" ] && return 0
			;;
		esac
	done <"$PLACEHOLDERS"
	return 1
}

# Senha por Docker secret: o /run.sh também entende o __FILE, mas a
# conferência e a gravação precisam acontecer antes dele.
if [ -n "${GF_SECURITY_ADMIN_PASSWORD__FILE:-}" ]; then
	[ -z "${GF_SECURITY_ADMIN_PASSWORD:-}" ] ||
		fail "GF_SECURITY_ADMIN_PASSWORD e GF_SECURITY_ADMIN_PASSWORD__FILE não podem vir juntas"
	GF_SECURITY_ADMIN_PASSWORD=$(cat "$GF_SECURITY_ADMIN_PASSWORD__FILE") ||
		fail "não foi possível ler GF_SECURITY_ADMIN_PASSWORD__FILE"
	export GF_SECURITY_ADMIN_PASSWORD
	unset GF_SECURITY_ADMIN_PASSWORD__FILE
fi
password=${GF_SECURITY_ADMIN_PASSWORD:-}
generate="gere uma com: openssl rand -base64 24"

[ -n "$password" ] || fail "defina GRAFANA_PASSWORD no .env ($generate)"
[ -r "$PLACEHOLDERS" ] ||
	fail "lista de valores de exemplo ausente em $PLACEHOLDERS (confira os volumes do grafana no docker-compose.yml)"
if is_placeholder "$password"; then
	fail "GRAFANA_PASSWORD é um valor de exemplo ou um padrão conhecido ($generate)"
fi
[ "${#password}" -ge "$MIN_LENGTH" ] ||
	fail "GRAFANA_PASSWORD precisa de ao menos $MIN_LENGTH caracteres ($generate)"
case $password in
*'
'*) fail "GRAFANA_PASSWORD não pode ter quebra de linha ($generate)" ;;
esac

# Grava a senha no banco. A senha vai pelo stdin, fora da lista de processos.
# A saída (as migrations do banco, centenas de linhas) só aparece se falhar.
if ! output=$(printf '%s\n' "$password" | grafana cli \
	--homepath "$GF_PATHS_HOME" \
	--config "$GF_PATHS_CONFIG" \
	--configOverrides "cfg:default.paths.data=$GF_PATHS_DATA cfg:default.paths.logs=$GF_PATHS_LOGS cfg:default.paths.plugins=$GF_PATHS_PLUGINS cfg:default.paths.provisioning=$GF_PATHS_PROVISIONING" \
	admin reset-admin-password --password-from-stdin 2>&1); then
	printf '%s\n' "$output" | tail -n 20 >&2
	fail "não foi possível gravar GRAFANA_PASSWORD no admin do Grafana (saída acima)"
fi
echo "grafana: senha do admin conferida e aplicada a partir de GRAFANA_PASSWORD"

exec "$GRAFANA_RUN" "$@"
