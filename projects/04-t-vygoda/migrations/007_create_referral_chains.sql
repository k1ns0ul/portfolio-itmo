CREATE TABLE IF NOT EXISTS referral_chains (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    referrer_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    level       SMALLINT NOT NULL CHECK (level IN (1, 2, 3)),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, referrer_id, level)
);

CREATE INDEX IF NOT EXISTS idx_chains_user     ON referral_chains (user_id);
CREATE INDEX IF NOT EXISTS idx_chains_referrer ON referral_chains (referrer_id);
