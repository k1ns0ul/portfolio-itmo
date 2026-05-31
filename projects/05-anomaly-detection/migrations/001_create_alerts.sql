CREATE DATABASE IF NOT EXISTS anomalies;

CREATE TABLE IF NOT EXISTS anomalies.alerts (
    id                String,
    tx_id             String,
    client_id         String,
    score             Float64,
    iforest_flag      UInt8,
    autoencoder_score Float64,
    level             String,
    created_at        DateTime
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(created_at)
ORDER BY (created_at, client_id);

CREATE INDEX IF NOT EXISTS idx_alerts_client ON anomalies.alerts (client_id) TYPE bloom_filter GRANULARITY 4;
CREATE INDEX IF NOT EXISTS idx_alerts_level  ON anomalies.alerts (level) TYPE set(8) GRANULARITY 4;
