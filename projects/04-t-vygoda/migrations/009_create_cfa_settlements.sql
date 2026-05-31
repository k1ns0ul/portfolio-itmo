CREATE TABLE IF NOT EXISTS cfa_settlements (
    id           BIGSERIAL PRIMARY KEY,
    purchase_id  BIGINT NOT NULL REFERENCES purchases(id) ON DELETE RESTRICT,
    partner_id   BIGINT NOT NULL REFERENCES partners(id) ON DELETE RESTRICT,
    debtor_type  TEXT NOT NULL CHECK (debtor_type IN ('bank', 'partner')),
    amount       NUMERIC(12,2) NOT NULL,
    status       TEXT NOT NULL CHECK (status IN ('created', 'confirmed', 'settled')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    settled_at   TIMESTAMPTZ,
    UNIQUE (purchase_id)
);

CREATE INDEX IF NOT EXISTS idx_cfa_partner_status ON cfa_settlements (partner_id, status);
CREATE INDEX IF NOT EXISTS idx_cfa_created       ON cfa_settlements (created_at DESC);
