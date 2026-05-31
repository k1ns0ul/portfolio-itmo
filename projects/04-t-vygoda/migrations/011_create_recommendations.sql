CREATE TABLE IF NOT EXISTS recommendations (
    id            BIGSERIAL PRIMARY KEY,
    user_id       BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    promo_id      BIGINT NOT NULL REFERENCES promos(id) ON DELETE CASCADE,
    score         REAL NOT NULL,
    reason        TEXT,
    generated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, promo_id)
);

CREATE INDEX IF NOT EXISTS idx_recs_user_score ON recommendations (user_id, score DESC);
CREATE INDEX IF NOT EXISTS idx_recs_generated  ON recommendations (generated_at);
