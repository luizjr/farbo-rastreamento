-- Um veículo, um rastreador, uma assinatura: a assinatura passa a apontar
-- para o veículo que ela cobre. Tudo entra pelo mesmo fluxo (veículo →
-- rastreador → assinatura); acabam as "vagas" soltas.
ALTER TABLE subscriptions ADD COLUMN vehicle_id UUID REFERENCES vehicles (id) ON DELETE SET NULL;

-- Um veículo tem no máximo uma assinatura ativa.
CREATE UNIQUE INDEX idx_subscriptions_vehicle_active ON subscriptions (vehicle_id)
    WHERE status = 'ACTIVE';

-- Dados anteriores: casa as assinaturas ativas com os veículos do mesmo
-- cliente, na ordem de cadastro. O que sobrar (assinatura sem veículo ou
-- veículo sem assinatura) aparece na ficha para a central resolver.
WITH subs AS (
    SELECT id, customer_id,
           row_number() OVER (PARTITION BY customer_id ORDER BY created_at, id) AS n
    FROM subscriptions
    WHERE status = 'ACTIVE'
), vehs AS (
    SELECT id, owner_id,
           row_number() OVER (PARTITION BY owner_id ORDER BY created_at, id) AS n
    FROM vehicles
    WHERE owner_id IS NOT NULL
)
UPDATE subscriptions s
SET vehicle_id = vehs.id
FROM subs
JOIN vehs ON vehs.owner_id = subs.customer_id AND vehs.n = subs.n
WHERE s.id = subs.id;
