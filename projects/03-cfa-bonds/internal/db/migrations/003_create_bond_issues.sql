CREATE TABLE IF NOT EXISTS bond_issues (
    id               UUID PRIMARY KEY,
    issuer_id        UUID NOT NULL REFERENCES issuers(id),
    name             TEXT NOT NULL,
    isin             TEXT NOT NULL UNIQUE,
    nominal          NUMERIC(30,8) NOT NULL,
    coupon_rate      NUMERIC(12,8) NOT NULL,
    coupon_frequency INTEGER NOT NULL CHECK (coupon_frequency IN (1,2,4)),
    issue_date       DATE NOT NULL,
    maturity_date    DATE NOT NULL,
    total_quantity   BIGINT NOT NULL CHECK (total_quantity > 0),
    placed_quantity  BIGINT NOT NULL DEFAULT 0,
    status           TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft','placement','active','matured','cancelled')),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_issues_status ON bond_issues (status);
CREATE INDEX IF NOT EXISTS idx_issues_issuer_status ON bond_issues (issuer_id, status);
CREATE INDEX IF NOT EXISTS idx_issues_maturity_status ON bond_issues (maturity_date, status);
