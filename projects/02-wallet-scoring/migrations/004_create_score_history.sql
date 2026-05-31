CREATE TABLE IF NOT EXISTS wallets.score_history (
    wallet    String,
    score     Float32,
    category  String,
    ts        DateTime
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(ts)
ORDER BY (wallet, ts);
