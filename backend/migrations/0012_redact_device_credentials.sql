-- Credenciais dos rastreadores fora do histórico (issue #1).
--
-- Até esta versão o texto de cada comando era gravado como saiu para o
-- aparelho — com a senha de comando dentro (no GT06, "DYD,<senha>#") — e a
-- resposta do aparelho, que alguns firmwares ecoam, ia inteira para o
-- registro e para a auditoria. O backend agora grava tudo já redigido (***,
-- ver internal/devices/secrets.go); esta migration limpa o que ficou para
-- trás em device_commands (payload, response, error) e na auditoria
-- (metadata.payload / response / reason):
--
--   1. as senhas atuais de cada aparelho (de comando e APN), onde aparecerem
--      — sem diferenciar maiúsculas quando a senha é só letras e números;
--   2. senhas antigas, já trocadas: nos comandos do GT06 que levam senha a
--      posição dela é conhecida ("CMD,<senha>#" e "TIMER,<senha>,<s>#").
--
-- Senha antiga no meio de um override não tem como ser reconhecida aqui: por
-- isso as senhas que já passaram por comandos devem ser trocadas (README,
-- "Credenciais dos rastreadores"). Rodar de novo não muda nada.

CREATE FUNCTION farbo_redact_secret_0012(txt TEXT, secret TEXT)
RETURNS TEXT LANGUAGE sql IMMUTABLE AS $$
    SELECT CASE
        WHEN txt IS NULL OR COALESCE(secret, '') = '' THEN txt
        -- Só letras e números: vale como expressão regular, e o 'i' pega o
        -- eco em outra caixa.
        WHEN secret ~ '^[A-Za-z0-9]+$' THEN regexp_replace(txt, secret, '***', 'gi')
        ELSE replace(replace(replace(txt, secret, '***'), upper(secret), '***'), lower(secret), '***')
    END
$$;

CREATE FUNCTION farbo_redact_0012(txt TEXT, command_password TEXT, apn_password TEXT)
RETURNS TEXT LANGUAGE sql IMMUTABLE AS $$
    SELECT regexp_replace(
        regexp_replace(
            farbo_redact_secret_0012(farbo_redact_secret_0012(txt, command_password), apn_password),
            '(DYD|HFYD|WHERE|STATUS|RESET),[A-Za-z0-9]{1,16}#', '\1,***#', 'gi'),
        '(TIMER),[A-Za-z0-9]{1,16},([0-9]+#)', '\1,***,\2', 'gi')
$$;

WITH redacted AS (
    SELECT c.id,
           farbo_redact_0012(c.payload,  d.command_password, d.apn_password) AS payload,
           farbo_redact_0012(c.response, d.command_password, d.apn_password) AS response,
           farbo_redact_0012(c.error,    d.command_password, d.apn_password) AS error
    FROM device_commands c
    JOIN devices d ON d.id = c.device_id
)
UPDATE device_commands c
SET payload = r.payload, response = r.response, error = r.error
FROM redacted r
WHERE c.id = r.id
  AND (c.payload IS DISTINCT FROM r.payload
       OR c.response IS DISTINCT FROM r.response
       OR c.error IS DISTINCT FROM r.error);

-- Auditoria de aparelho já excluído (device_id nulo) passa só pela regra 2.
WITH redacted AS (
    SELECT a.id, a.metadata AS before,
           (SELECT jsonb_object_agg(e.key,
                       CASE WHEN e.key IN ('payload', 'response', 'reason')
                                 AND jsonb_typeof(e.value) = 'string'
                            THEN to_jsonb(farbo_redact_0012(e.value #>> '{}',
                                                            d.command_password, d.apn_password))
                            ELSE e.value
                       END)
            FROM jsonb_each(a.metadata) e) AS after
    FROM audit_logs a
    LEFT JOIN devices d ON d.id = a.device_id
    WHERE jsonb_typeof(a.metadata) = 'object'
      AND (a.metadata ? 'payload' OR a.metadata ? 'response' OR a.metadata ? 'reason')
)
UPDATE audit_logs a
SET metadata = r.after
FROM redacted r
WHERE a.id = r.id AND r.after IS DISTINCT FROM r.before;

DROP FUNCTION farbo_redact_0012(TEXT, TEXT, TEXT);
DROP FUNCTION farbo_redact_secret_0012(TEXT, TEXT);
