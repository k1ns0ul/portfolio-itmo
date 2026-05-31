CREATE DATABASE IF NOT EXISTS wallets;

CREATE TABLE IF NOT EXISTS wallets.transactions (
    signature     String,
    slot          UInt64,
    block_time    DateTime,
    fee           UInt64,
    sender        String,
    receiver      String,
    amount        UInt64,
    program_id    String,
    swap_kind     String DEFAULT '',
    success       UInt8,
    raw_accounts  Array(String),
    raw_data      String
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(block_time)
ORDER BY (block_time, slot, signature);

CREATE INDEX IF NOT EXISTS idx_sender   ON wallets.transactions (sender)     TYPE bloom_filter GRANULARITY 4;
CREATE INDEX IF NOT EXISTS idx_receiver ON wallets.transactions (receiver)   TYPE bloom_filter GRANULARITY 4;
CREATE INDEX IF NOT EXISTS idx_program  ON wallets.transactions (program_id) TYPE set(64) GRANULARITY 4;
