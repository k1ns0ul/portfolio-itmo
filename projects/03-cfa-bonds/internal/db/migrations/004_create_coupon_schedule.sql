CREATE TABLE IF NOT EXISTS coupon_schedule (
    id           UUID PRIMARY KEY,
    issue_id     UUID NOT NULL REFERENCES bond_issues(id) ON DELETE CASCADE,
    sequence_num INTEGER NOT NULL,
    payment_date DATE NOT NULL,
    amount       NUMERIC(30,8) NOT NULL,
    status       TEXT NOT NULL DEFAULT 'scheduled'
        CHECK (status IN ('scheduled','processing','paid','failed')),
    paid_at      TIMESTAMPTZ,
    UNIQUE (issue_id, sequence_num)
);

CREATE INDEX IF NOT EXISTS idx_coupon_due ON coupon_schedule (issue_id, status, payment_date);

CREATE TABLE IF NOT EXISTS coupon_payments (
    id          UUID PRIMARY KEY,
    coupon_id   UUID NOT NULL REFERENCES coupon_schedule(id) ON DELETE CASCADE,
    investor_id UUID NOT NULL REFERENCES investors(id),
    issue_id    UUID NOT NULL REFERENCES bond_issues(id) ON DELETE CASCADE,
    amount      NUMERIC(30,8) NOT NULL,
    paid_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_coupon_payments_investor ON coupon_payments (investor_id);
CREATE INDEX IF NOT EXISTS idx_coupon_payments_issue ON coupon_payments (issue_id);
