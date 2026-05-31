CREATE MATERIALIZED VIEW IF NOT EXISTS mv_daily_volume AS
SELECT
    issue_id,
    date_trunc('day', settled_at) AS day,
    count(*)                       AS trade_count,
    sum(quantity)                  AS total_quantity,
    sum(total_amount)              AS total_volume
FROM trades
WHERE status = 'settled' AND settled_at IS NOT NULL
GROUP BY issue_id, date_trunc('day', settled_at);

CREATE UNIQUE INDEX IF NOT EXISTS idx_mv_daily_volume ON mv_daily_volume (issue_id, day);

CREATE MATERIALIZED VIEW IF NOT EXISTS mv_issue_stats AS
SELECT
    bi.id                         AS issue_id,
    bi.isin,
    bi.status,
    COALESCE(count(t.id), 0)      AS lifetime_trades,
    COALESCE(sum(t.total_amount), 0) AS lifetime_volume
FROM bond_issues bi
LEFT JOIN trades t ON t.issue_id = bi.id AND t.status = 'settled'
GROUP BY bi.id, bi.isin, bi.status;

CREATE UNIQUE INDEX IF NOT EXISTS idx_mv_issue_stats ON mv_issue_stats (issue_id);
