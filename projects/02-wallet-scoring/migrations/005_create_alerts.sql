CREATE TABLE IF NOT EXISTS wallets.alerts (
    id          String,
    level       String,
    wallet      String,
    rule        String,
    message     String,
    payload     String,
    created_at  DateTime
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(created_at)
ORDER BY (created_at, wallet);
