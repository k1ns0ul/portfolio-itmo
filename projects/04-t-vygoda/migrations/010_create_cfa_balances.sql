CREATE TABLE IF NOT EXISTS cfa_balances (
    partner_id   BIGINT PRIMARY KEY REFERENCES partners(id) ON DELETE CASCADE,
    bank_owes    NUMERIC(14,2) NOT NULL DEFAULT 0,
    partner_owes NUMERIC(14,2) NOT NULL DEFAULT 0,
    net_balance  NUMERIC(14,2) GENERATED ALWAYS AS (bank_owes - partner_owes) STORED,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS cfa_reconciliations (
    id           BIGSERIAL PRIMARY KEY,
    partner_id   BIGINT NOT NULL REFERENCES partners(id) ON DELETE CASCADE,
    settled_amount NUMERIC(14,2) NOT NULL,
    settled_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_cfa_recon_partner ON cfa_reconciliations (partner_id, settled_at DESC);
