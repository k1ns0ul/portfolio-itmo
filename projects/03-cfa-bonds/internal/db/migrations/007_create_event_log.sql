CREATE TABLE IF NOT EXISTS event_log (
    id          UUID NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id   UUID NOT NULL,
    event_type  TEXT NOT NULL,
    payload     JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

CREATE TABLE IF NOT EXISTS event_log_default PARTITION OF event_log DEFAULT;

CREATE INDEX IF NOT EXISTS idx_event_entity ON event_log (entity_type, entity_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_event_type ON event_log (event_type, created_at DESC);
