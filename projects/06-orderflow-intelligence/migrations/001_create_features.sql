CREATE DATABASE IF NOT EXISTS orderflow;

CREATE TABLE IF NOT EXISTS orderflow.swaps (
    signature       String,
    slot            UInt64,
    block_time      DateTime,
    dex             String,
    pool_address    String,
    pair            String,
    token_in        String,
    token_out       String,
    amount_in       UInt64,
    amount_out      UInt64,
    price           Float64,
    direction       String,
    sender          String
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(block_time)
ORDER BY (block_time, pair, slot);

CREATE TABLE IF NOT EXISTS orderflow.feature_windows (
    pair              String,
    interval_sec      UInt32,
    window_start      DateTime,
    window_end        DateTime,
    ofi               Float64,
    vpin              Float64,
    price_impact      Float64,
    avg_swap_size     Float64,
    buy_ratio         Float64,
    cumulative_volume Float64,
    price_range       Float64,
    price_open        Float64,
    price_close       Float64,
    swap_count        UInt32
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(window_end)
ORDER BY (pair, interval_sec, window_end);

CREATE TABLE IF NOT EXISTS orderflow.predictions (
    pair        String,
    window_end  DateTime,
    direction   String,
    confidence  Float64,
    xgb_prob    Float64,
    lstm_prob   Float64,
    created_at  DateTime
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(created_at)
ORDER BY (pair, window_end);
