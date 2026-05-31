CREATE TABLE IF NOT EXISTS trades (
    id               UUID PRIMARY KEY,
    issue_id         UUID NOT NULL REFERENCES bond_issues(id) ON DELETE CASCADE,
    seller_id        UUID NOT NULL REFERENCES investors(id),
    buyer_id         UUID NOT NULL REFERENCES investors(id),
    quantity         BIGINT NOT NULL CHECK (quantity > 0),
    price            NUMERIC(30,8) NOT NULL,
    accrued_interest NUMERIC(30,8) NOT NULL DEFAULT 0,
    total_amount     NUMERIC(30,8) NOT NULL DEFAULT 0,
    status           TEXT NOT NULL DEFAULT 'submitted'
        CHECK (status IN ('submitted','settled','failed')),
    failure_reason   TEXT NOT NULL DEFAULT '',
    submitted_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    settled_at       TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_trades_issue_submitted ON trades (issue_id, submitted_at);
CREATE INDEX IF NOT EXISTS idx_trades_seller ON trades (seller_id);
CREATE INDEX IF NOT EXISTS idx_trades_buyer ON trades (buyer_id);
CREATE INDEX IF NOT EXISTS idx_trades_status ON trades (status);
