CREATE DATABASE IF NOT EXISTS wallets;

CREATE TABLE IF NOT EXISTS wallets.token_balances (
    wallet      String,
    mint        String,
    symbol      String,
    amount      Float64,
    last_price  Float64,
    updated_at  DateTime
) ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (wallet, mint);

CREATE TABLE IF NOT EXISTS wallets.token_pnl (
    wallet         String,
    mint           String,
    avg_buy_price  Float64,
    current_price  Float64,
    quantity       Float64,
    realized_pnl   Float64,
    unrealized_pnl Float64,
    updated_at     DateTime
) ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (wallet, mint);
