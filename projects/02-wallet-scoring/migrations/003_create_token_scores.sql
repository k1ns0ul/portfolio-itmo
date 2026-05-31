CREATE TABLE IF NOT EXISTS wallets.token_scores (
    mint        String,
    category    String,
    confidence  Float32,
    risk_score  Float32,
    holders     UInt32,
    volume_24h  Float64,
    updated_at  DateTime
) ENGINE = ReplacingMergeTree(updated_at)
ORDER BY mint;
