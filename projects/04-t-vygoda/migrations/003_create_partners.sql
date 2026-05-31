CREATE TABLE IF NOT EXISTS partners (
    id            BIGSERIAL PRIMARY KEY,
    name          TEXT NOT NULL,
    logo_url      TEXT,
    cpa_percent   NUMERIC(5,2) NOT NULL DEFAULT 0,
    contact_email TEXT,
    active        BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_partners_active ON partners (active);
