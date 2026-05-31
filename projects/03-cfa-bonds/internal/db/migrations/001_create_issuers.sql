CREATE TABLE IF NOT EXISTS issuers (
    id            UUID PRIMARY KEY,
    name          TEXT NOT NULL,
    inn           TEXT NOT NULL UNIQUE,
    ogrn          TEXT NOT NULL DEFAULT '',
    contact_email TEXT NOT NULL DEFAULT '',
    active        BOOLEAN NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_issuers_active ON issuers (active);
