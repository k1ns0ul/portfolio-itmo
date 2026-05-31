CREATE TABLE IF NOT EXISTS referral_bonuses (
    id          BIGSERIAL PRIMARY KEY,
    chain_id    BIGINT NOT NULL REFERENCES referral_chains(id) ON DELETE CASCADE,
    purchase_id BIGINT NOT NULL REFERENCES purchases(id) ON DELETE CASCADE,
    referrer_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount      NUMERIC(12,2) NOT NULL,
    level       SMALLINT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'credited',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (purchase_id, referrer_id, level)
);

CREATE INDEX IF NOT EXISTS idx_bonuses_referrer ON referral_bonuses (referrer_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_bonuses_purchase ON referral_bonuses (purchase_id);
