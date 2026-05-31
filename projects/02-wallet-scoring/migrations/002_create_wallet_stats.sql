CREATE TABLE IF NOT EXISTS wallets.wallet_stats (
    wallet                String,
    tx_count              UInt64,
    first_seen            DateTime,
    last_seen             DateTime,
    unique_counterparties UInt32,
    avg_tx_amount         Float64,
    median_tx_amount      Float64,
    herfindahl_index      Float64,
    smart_contract_ratio  Float32,
    velocity_per_hour     Float64,
    dormancy_days         Float64,
    score                 Float32,
    category              String,
    updated_at            DateTime
) ENGINE = ReplacingMergeTree(updated_at)
ORDER BY wallet;
