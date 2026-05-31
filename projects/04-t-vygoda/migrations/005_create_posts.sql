CREATE TABLE IF NOT EXISTS posts (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title        TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    image_url    TEXT,
    price_before NUMERIC(12,2),
    price_after  NUMERIC(12,2),
    promo_id     BIGINT REFERENCES promos(id) ON DELETE SET NULL,
    category_id  BIGINT REFERENCES categories(id) ON DELETE SET NULL,
    likes_count  INT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_posts_user      ON posts (user_id);
CREATE INDEX IF NOT EXISTS idx_posts_category  ON posts (category_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_posts_created   ON posts (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_posts_likes     ON posts (likes_count DESC);
