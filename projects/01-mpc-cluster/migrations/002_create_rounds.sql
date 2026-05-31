CREATE TABLE IF NOT EXISTS rounds (
    session_id   TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    number       INTEGER NOT NULL,
    phase        TEXT NOT NULL,
    status       TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    PRIMARY KEY (session_id, number)
);
