CREATE TABLE IF NOT EXISTS anomalies.features (
    tx_id                     String,
    client_id                 String,
    amount                    Float64,
    avg_amount_1h             Float64,
    avg_amount_24h            Float64,
    unique_counterparties_24h UInt32,
    z_score                   Float64,
    time_since_last_tx        Float64,
    night_flag                UInt8,
    frequency_score           Float64,
    ts                        DateTime
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(ts)
ORDER BY (ts, client_id);
