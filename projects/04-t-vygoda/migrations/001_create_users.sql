CREATE TABLE IF NOT EXISTS users (
    id             BIGSERIAL PRIMARY KEY,
    phone          TEXT NOT NULL UNIQUE,
    name           TEXT NOT NULL,
    email          TEXT,
    avatar_url     TEXT,
    referral_code  TEXT NOT NULL UNIQUE,
    referred_by    BIGINT REFERENCES users(id) ON DELETE SET NULL,
    level          SMALLINT NOT NULL DEFAULT 1,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_users_referred_by ON users (referred_by);
CREATE INDEX IF NOT EXISTS idx_users_phone        ON users (phone);
