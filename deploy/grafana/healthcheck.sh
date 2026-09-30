#!/bin/sh
# Healthcheck do Grafana: o servidor responde e a senha de GRAFANA_PASSWORD
# entra de fato como admin (confere a gravação feita pelo entrypoint.sh).
# Credencial pelo stdin do curl (-H @-), fora da lista de processos.
set -eu

user=${GF_SECURITY_ADMIN_USER:-admin}
if [ -n "${GF_SECURITY_ADMIN_PASSWORD__FILE:-}" ]; then
	password=$(cat "$GF_SECURITY_ADMIN_PASSWORD__FILE")
else
	password=${GF_SECURITY_ADMIN_PASSWORD:-}
fi
[ -n "$password" ] || exit 1

auth=$(printf '%s:%s' "$user" "$password" | base64 | tr -d '\n')
printf 'Authorization: Basic %s\n' "$auth" |
	curl -fsS -o /dev/null --max-time 5 -H @- \
		"http://127.0.0.1:${GF_SERVER_HTTP_PORT:-3000}/api/user"
