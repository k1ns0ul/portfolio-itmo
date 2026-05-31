CREATE TABLE IF NOT EXISTS investors (
    id             UUID PRIMARY KEY,
    name           TEXT NOT NULL,
    type           TEXT NOT NULL CHECK (type IN ('individual','legal_entity')),
    account_number TEXT NOT NULL UNIQUE,
    balance        NUMERIC(30,8) NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_investors_account ON investors (account_number);
