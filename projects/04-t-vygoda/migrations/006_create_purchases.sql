CREATE TABLE IF NOT EXISTS purchases (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    promo_id     BIGINT NOT NULL REFERENCES promos(id) ON DELETE RESTRICT,
    partner_id   BIGINT NOT NULL REFERENCES partners(id) ON DELETE RESTRICT,
    amount       NUMERIC(12,2) NOT NULL,
    cpa_amount   NUMERIC(12,2) NOT NULL DEFAULT 0,
    status       TEXT NOT NULL CHECK (status IN ('pending', 'confirmed', 'cancelled')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    confirmed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_purchases_user_created    ON purchases (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_purchases_partner_created ON purchases (partner_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_purchases_status          ON purchases (status);
