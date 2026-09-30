# Arquitetura

## Ideia central

Uma direção de dependência só, e uma fronteira que não se atravessa: **o
domínio não conhece bytes**.

```text
TKSTAR / GT06
     ↓
TCP (sessão, enquadramento, limites)
     ↓
Protocol Adapter  ──→ TrackerMessage (normalizada)
     ↓
Tracking (estado, histórico, eventos)
     ↓
PostgreSQL / Redis
     ↓
REST + WebSocket
     ↓
React
```

Tudo o que é específico de fabricante vive em `internal/protocols/`. Acima
disso ninguém sabe o que é `0x78`, CRC-ITU ou `DYD#`. Trocar de fabricante é
escrever um adaptador — nada mais muda.

## Pacotes

```text
backend/
├── cmd/
│   ├── server/          monta tudo e sobe HTTP, TCP e as rotinas periódicas
│   └── tksim/           simulador de rastreador
└── internal/
    ├── protocols/       TrackerProtocol, TrackerMessage, Command, registry
    │   ├── gt06/        binário Concox            (DOCUMENTED)
    │   ├── h02/         binário + texto, TKSTAR   (ASSUMED)
    │   └── tkstar/      texto V1, V1.8, V4        (ASSUMED / UNKNOWN)
    ├── tcp/             servidor, sessão, ConnectionManager
    ├── tracking/        posições, estado, simplificação, ingestor
    ├── devices/         cadastro e status dos rastreadores, representação por perfil e redação das senhas
    ├── vehicles/        cadastro dos veículos
    ├── events/          eventos derivados
    ├── commands/        ciclo de vida do comando + regra de segurança
    ├── geofences/       cercas circulares
    ├── audit/           trilha de auditoria
    ├── auth/            JWT, refresh token, RBAC, redefinição de senha
    ├── mail/            SMTP e os e-mails da conta
    ├── billing/         assinaturas, faturas, geração mensal, suspensão
    ├── orders/          Novo veículo: veículo → rastreador → assinatura (+ fatura do equipamento), numa transação
    ├── installers/      prestadores de instalação recomendados (lista pública da landing)
    ├── addresses/       endereço de entrega do cliente (copiado na assinatura ao contratar)
    ├── fulfillment/     pedidos: linhas do tempo do chip M2M e do rastreador, etiqueta e rastreio
    ├── melhorenvio/     cliente da API do Melhor Envios (OAuth, frete, etiqueta, rastreio, webhook)
    ├── retention/       por quantos dias guardar o histórico de cada veículo (7/14/30) e a limpeza
    ├── payments/        Pix das faturas e baixa automática
    │   └── abacatepay/  cliente da API v2 e verificação de webhook
    ├── websocket/       hub, cliente, ponte Redis (mensagens assinadas)
    ├── realtime/        confere o "data" de cada evento vindo do Redis
    ├── leakcheck/       (testes) procura credenciais em respostas e eventos, em todas as codificações
    ├── api/             rotas REST e middlewares
    ├── database/        pool pgx e migrations
    ├── telemetry/       log estruturado, métricas, tracing
    └── config/          variáveis de ambiente
```

## Como as dependências ficam acíclicas

Três pontos merecem atenção, porque é onde um ciclo apareceria:

**1. `tcp` não conhece o domínio.** Ele declara a interface `Ingestor` e
recebe uma implementação. Quem implementa é `tracking.Ingestor`.

**2. `commands` não conhece `tcp` nem `tracking`.** Ele declara o que precisa:

```go
type Sender interface{ Send(imei string, payload []byte) error }
type TelemetryProvider interface{ Snapshot(ctx, deviceID) (Snapshot, error) }
type Store interface{ /* … */ }
```

`tcp.Manager` satisfaz `Sender` (adaptado em `main.go`), e
`tracking.SnapshotProvider` satisfaz `TelemetryProvider`. Como efeito
colateral bem-vindo, a regra de corte de motor é testável sem banco e sem rede.

**3. `tracking` importa `commands`**, para entregar o ACK. Como `commands` não
importa `tracking`, não há ciclo.

## O caminho de uma posição

```text
1. socket entrega bytes            → session.buf (acumula)
2. registry.Detect(buf)            → escolhe o adaptador (uma vez por conexão)
3. proto.NextFrame(buf)            → recorta um quadro completo
4. proto.Parse(frame)              → TrackerMessage
5. sessão escreve o ACK            → antes de processar: firmware não espera
6. ingestor.HandleMessage
   ├─ resolve o dispositivo pelo IMEI (cache de 60 s)
   ├─ recusa a sessão se o IMEI não estiver cadastrado
   ├─ grava o último contato (no máximo 1 UPDATE a cada 15 s)
   ├─ atualiza device_states e compara com o anterior → eventos
   ├─ grava a posição em positions
   ├─ avalia excesso de velocidade (com histerese) e cercas
   └─ publica no WebSocket
```

Nada nesse caminho bloqueia por causa de um consumidor lento: o hub descarta
mensagem de cliente entupido, a persistência do estado é assíncrona e o Redis
recebe em segundo plano.

## O caminho de um comando

```text
1. POST /api/vehicles/:id/commands/engine-cut
2. commands.Send
   ├─ é ENGINE_CUT? → consulta a última posição
   │    ├─ sem posição            → REJECTED
   │    ├─ posição velha demais   → REJECTED
   │    └─ acima do limite        → REJECTED
   ├─ sorteia a chave de correlação
   ├─ codifica pelo adaptador do dispositivo (override tem precedência)
   ├─ grava PENDING + auditoria + evento
   ├─ SENDING → escreve na sessão TCP → SENT, com prazo de ACK
   └─ resposta HTTP 202 (ou 409 se recusado)
3. o rastreador responde → Parse → KindCommandAck
4. commands.HandleAck casa pela chave (ou, em protocolo de texto, pelo comando
   aberto mais antigo — e isso fica escrito no registro)
5. ACKNOWLEDGED | FAILED → auditoria + evento + WebSocket
6. varredura periódica fecha o que passou do prazo como TIMEOUT
```

## Decisões que valem explicar

**Heartbeat não vira posição.** Ele atualiza `device_states`. Criar uma linha
em `positions` reaproveitando a coordenada anterior daria um fix de GPS que o
aparelho nunca enviou.

**`gps_timestamp` e `received_at` são colunas separadas.** Rastreador com
memória descarrega posições antigas ao reconectar. Sem os dois campos, o
trajeto fica errado — e a regra do corte de motor usa `received_at` de
propósito: o que importa para autorizar o corte é há quanto tempo o servidor
teve notícia do veículo, não o que o relógio do aparelho afirma.

**Histórico é amostrado no banco.** Uma janela de 7 dias pode ter centenas de
milhares de pontos. A consulta numera as linhas, devolve 1 a cada N (mantendo
a primeira e a última) e diz na resposta que amostrou. O traçado ainda passa
por Douglas-Peucker para o mapa. O dado bruto continua intacto e sai por
paginação com cursor.

**Status não é a conexão TCP.** Um socket aberto sem tráfego não significa
aparelho vivo. `ONLINE`/`STALE`/`OFFLINE` saem do último pacote recebido,
recalculados por varredura periódica.

**O protocolo detectado sobrescreve o cadastrado.** Quem decide como montar o
comando é o que o aparelho fala, não o que foi digitado no formulário.

**Estado derivado é persistido.** `device_states` guarda excesso de
velocidade, cercas em que o veículo está e ACC. Sem isso, um restart geraria
uma enxurrada de eventos falsos de entrada em cerca.

## Fronteiras de segurança

| Onde | O quê |
| --- | --- |
| TCP | limite de tamanho de pacote, timeout de leitura, teto de conexões, sessão isolada de pânico |
| Ingestão | IMEI desconhecido encerra a sessão; pacote inválido é capturado, não derruba a conexão |
| API | JWT com rotação de refresh token, RBAC por rota, rate limit por IP (mais apertado no login), corpo limitado, campo desconhecido recusado |
| Painel do cliente | perfil `customer` filtrado no ponto único `vehicleFromURL` (veículo alheio = 404), rotas da central com `RequireRole`, WebSocket filtrado por dono (índice em memória), rastreador sem credenciais na resposta, cota de veículos checada na mesma transação da inserção, suspensão por atraso (402) |
| Pagamentos | chave da AbacatePay só no ambiente (nunca em log), webhook com segredo na URL + HMAC do corpo + idempotência por id, baixa só depois de reconsultar o Pix na API, cliente só gera/consulta Pix das próprias faturas |
| Redefinição de senha | mesma resposta com ou sem conta, e-mail enviado fora da requisição (sem diferença de tempo), token de 256 bits guardado como hash, uso único, validade curta, um pedido por minuto por conta, troca revoga todas as sessões |
| Comandos | trava de velocidade no backend, texto sanitizado contra injeção, `CUSTOM` restrito a admin, auditoria de tudo |
| Credenciais dos rastreadores | senha APN e senha de comando só de escrita: nenhuma leitura as devolve (`devices.View`, lista fechada por perfil); o aparelho recebe o comando real, mas histórico, auditoria, WebSocket e provisionamento (só admin) recebem o texto com a senha trocada por `***` (`devices.RedactBytes`, antes de tornar o pacote legível) |
| Logs | IMEI mascarado; senha, token e credencial nunca registrados |

## Escala horizontal

Com `REDIS_ENABLED=true`, os eventos do WebSocket são replicados entre
instâncias por pub/sub, com a mensagem carregando a origem para não voltar
duplicada. O Redis é tratado como transporte não confiável: cada mensagem vai
assinada (HMAC-SHA256, chave derivada do `JWT_SECRET`) e com prazo de 2
minutos, e quem recebe refaz o `data` no tipo Go que o backend publica
(`realtime.Decode`) antes de entregar ao navegador. O que **não** escala assim é a sessão TCP: o comando só sai pela
instância onde o rastreador está conectado. Para várias instâncias, ou se usa
afinidade por IMEI no balanceador, ou se acrescenta um roteamento de comandos
por Redis — ponto de extensão ainda não implementado.
