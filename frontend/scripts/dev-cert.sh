#!/usr/bin/env bash
# Certificado HTTPS de desenvolvimento em que o celular confia.
#
# O certificado provisório do Vite (plugin-basic-ssl) abre o app no celular
# depois do aviso, mas o iPhone não baixa ícone nem telas de abertura ao
# "Adicionar à Tela de Início" e não aceita service worker (offline e
# notificações) sem um certificado confiável.
#
# Este script cria, em frontend/.certs (fora do git):
#   farbo-dev-ca.crt   autoridade certificadora de desenvolvimento — é ela que
#                      vai para o celular. Ela SÓ vale para endereços de rede
#                      local (10/8, 172.16/12, 192.168/16, 127/8), localhost e
#                      localtest.me (nameConstraints): instalada no celular,
#                      não serve para interceptar nenhum site da internet.
#   farbo-dev-ca.key   chave da autoridade — nunca sai desta máquina
#   dev-cert.pem/.key  certificado do servidor, usado pelo vite.config.ts
#
# Uso: npm run dev:cert            (detecta o IP da rede)
#      npm run dev:cert -- 192.168.0.20   (IPs extras)
set -euo pipefail

cd "$(dirname "$0")/.."
DIR=.certs
mkdir -p "$DIR"
chmod 700 "$DIR"

# IP da máquina na rede: o endereço que o sistema usaria para sair para a
# internet (o "connect" de UDP não envia nada).
LAN_IP=$(node -e "const s = require('dgram').createSocket('udp4'); s.on('error', () => process.exit(0)); s.connect(53, '1.1.1.1', () => { console.log(s.address().address); s.close(); });" 2>/dev/null || true)
IPS=("127.0.0.1" ${LAN_IP:+"$LAN_IP"} "$@")

if [[ ! -f "$DIR/farbo-dev-ca.key" ]]; then
  cat > "$DIR/ca.cnf" <<'CNF'
[req]
distinguished_name = dn
x509_extensions = v3_ca
prompt = no
[dn]
CN = Farbo - desenvolvimento local (somente rede local)
O = Farbo Rastreadores - desenvolvimento
[v3_ca]
basicConstraints = critical, CA:TRUE, pathlen:0
keyUsage = critical, keyCertSign, cRLSign
subjectKeyIdentifier = hash
nameConstraints = critical, permitted;IP:10.0.0.0/255.0.0.0, permitted;IP:172.16.0.0/255.240.0.0, permitted;IP:192.168.0.0/255.255.0.0, permitted;IP:127.0.0.0/255.0.0.0, permitted;DNS:localhost, permitted;DNS:localtest.me
CNF
  openssl req -x509 -new -newkey rsa:2048 -nodes -sha256 -days 825 \
    -keyout "$DIR/farbo-dev-ca.key" -out "$DIR/farbo-dev-ca.crt" -config "$DIR/ca.cnf" 2>/dev/null
  chmod 600 "$DIR/farbo-dev-ca.key"
  rm "$DIR/ca.cnf"
  echo "autoridade de desenvolvimento criada: $DIR/farbo-dev-ca.crt"
fi

SAN="DNS:localhost,DNS:farbo.localtest.me"
for ip in "${IPS[@]}"; do SAN="$SAN,IP:$ip"; done

cat > "$DIR/server.cnf" <<CNF
[req]
distinguished_name = dn
prompt = no
[dn]
CN = Farbo dev server
[ext]
basicConstraints = critical, CA:FALSE
keyUsage = critical, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = $SAN
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid
CNF
# O iPhone recusa certificado de servidor com validade maior que ~13 meses.
openssl req -new -newkey rsa:2048 -nodes -sha256 -keyout "$DIR/dev-key.pem" -out "$DIR/server.csr" -config "$DIR/server.cnf" 2>/dev/null
openssl x509 -req -in "$DIR/server.csr" -CA "$DIR/farbo-dev-ca.crt" -CAkey "$DIR/farbo-dev-ca.key" -CAcreateserial \
  -out "$DIR/dev-cert.pem" -days 397 -sha256 -extfile "$DIR/server.cnf" -extensions ext 2>/dev/null
chmod 600 "$DIR/dev-key.pem"
rm -f "$DIR/server.csr" "$DIR/server.cnf" "$DIR/farbo-dev-ca.srl"

openssl verify -CAfile "$DIR/farbo-dev-ca.crt" "$DIR/dev-cert.pem" >/dev/null
echo "certificado do servidor para: ${SAN//,/ }"
echo
echo "No iPhone (uma vez):"
echo "  1. Abra o arquivo $DIR/farbo-dev-ca.crt no celular e toque em Permitir."
echo "  2. Ajustes > Geral > VPN e Gerenciamento de Dispositivos > instale o perfil."
echo "  3. Ajustes > Geral > Sobre > Ajustes de Confiança de Certificados > ative a confiança total."
echo "Reinicie o npm run dev: o Vite passa a usar este certificado."
