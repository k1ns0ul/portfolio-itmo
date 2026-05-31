CREATE TABLE IF NOT EXISTS positions (
    id          UUID PRIMARY KEY,
    investor_id UUID NOT NULL REFERENCES investors(id),
    issue_id    UUID NOT NULL REFERENCES bond_issues(id) ON DELETE CASCADE,
    quantity    BIGINT NOT NULL DEFAULT 0 CHECK (quantity >= 0),
    avg_price   NUMERIC(30,8) NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (investor_id, issue_id)
);

CREATE INDEX IF NOT EXISTS idx_positions_issue ON positions (issue_id);
