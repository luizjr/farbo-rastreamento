-- Prestadores de instalação recomendados. A instalação é combinada e paga
-- direto a eles (não passa pela cobrança da plataforma); a central só
-- mantém a lista que aparece na landing page e no painel do cliente.
CREATE TABLE installers (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name             TEXT        NOT NULL,
    city             TEXT        NOT NULL DEFAULT '',
    -- Bairros ou regiões atendidas, em texto livre.
    service_area     TEXT        NOT NULL DEFAULT '',
    -- Só dígitos, com o 55 do Brasil (vira o link wa.me).
    whatsapp         TEXT        NOT NULL,
    serves_moto      BOOLEAN     NOT NULL DEFAULT TRUE,
    serves_car       BOOLEAN     NOT NULL DEFAULT TRUE,
    -- Valores de referência do prestador; nulos quando ele prefere combinar.
    price_moto_cents INTEGER     CHECK (price_moto_cents >= 0),
    price_car_cents  INTEGER     CHECK (price_car_cents >= 0),
    description      TEXT        NOT NULL DEFAULT '',
    -- Inativo continua cadastrado, mas some da landing e do painel.
    active           BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (serves_moto OR serves_car)
);
CREATE INDEX idx_installers_active ON installers (active, lower(city), lower(name));
