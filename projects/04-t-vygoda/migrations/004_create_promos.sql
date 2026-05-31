CREATE TABLE IF NOT EXISTS promos (
    id           BIGSERIAL PRIMARY KEY,
    partner_id   BIGINT NOT NULL REFERENCES partners(id) ON DELETE CASCADE,
    code         TEXT NOT NULL UNIQUE,
    discount     NUMERIC(10,2) NOT NULL,
    type         TEXT NOT NULL CHECK (type IN ('percent', 'fixed')),
    category_id  BIGINT REFERENCES categories(id) ON DELETE SET NULL,
    max_uses     INT NOT NULL DEFAULT 0,
    current_uses INT NOT NULL DEFAULT 0,
    expires_at   TIMESTAMPTZ,
    active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_promos_partner   ON promos (partner_id);
CREATE INDEX IF NOT EXISTS idx_promos_category  ON promos (category_id);
CREATE INDEX IF NOT EXISTS idx_promos_active    ON promos (active, expires_at);
