CREATE TABLE IF NOT EXISTS session_nodes (
    session_id    TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    node_id       INTEGER NOT NULL,
    addr          TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'registered',
    registered_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (session_id, node_id)
);
