from __future__ import annotations

import logging
from datetime import datetime
from typing import Iterable

import clickhouse_connect
import pandas as pd
from clickhouse_connect.driver.client import Client

from config import MLConfig

log = logging.getLogger(__name__)


class ClickHouseDB:
    def __init__(self, cfg: MLConfig) -> None:
        self.cfg = cfg
        self.client: Client = clickhouse_connect.get_client(
            host=cfg.clickhouse_host,
            port=cfg.clickhouse_port,
            username=cfg.clickhouse_user,
            password=cfg.clickhouse_password,
            database=cfg.clickhouse_db,
            connect_timeout=5,
        )

    def close(self) -> None:
        self.client.close()

    def fetch_training_data(self, since_days: int = 90) -> pd.DataFrame:
        q = """
            SELECT signature, slot, block_time, fee, sender, receiver, amount,
                   program_id, swap_kind, success
            FROM wallets.transactions
            WHERE block_time >= now() - INTERVAL {days:UInt32} DAY
              AND sender != ''
        """
        df = self.client.query_df(q, parameters={"days": since_days})
        log.info("fetched %d rows for training", len(df))
        return df

    def fetch_wallet_features(self, wallets: Iterable[str]) -> pd.DataFrame:
        wallets = list(wallets)
        if not wallets:
            return pd.DataFrame()
        q = """
            SELECT signature, slot, block_time, fee, sender, receiver, amount,
                   program_id, swap_kind, success
            FROM wallets.transactions
            WHERE sender IN {wallets:Array(String)}
               OR receiver IN {wallets:Array(String)}
        """
        return self.client.query_df(q, parameters={"wallets": wallets})

    def list_all_wallets(self, limit: int = 0) -> list[str]:
        q = "SELECT DISTINCT sender AS w FROM wallets.transactions WHERE sender != ''"
        if limit > 0:
            q += f" LIMIT {int(limit)}"
        rows = self.client.query(q).result_rows
        return [r[0] for r in rows]

    def write_scores(self, scores: pd.DataFrame) -> None:
        if scores.empty:
            return
        now = datetime.utcnow()
        rows = []
        for w, row in scores.iterrows():
            rows.append([
                str(w),
                int(row.get("tx_count", 0) or 0),
                row.get("first_seen", now),
                row.get("last_seen", now),
                int(row.get("unique_counterparties", 0) or 0),
                float(row.get("avg_tx_amount", 0.0) or 0.0),
                float(row.get("median_tx_amount", 0.0) or 0.0),
                float(row.get("herfindahl_index", 0.0) or 0.0),
                float(row.get("smart_contract_ratio", 0.0) or 0.0),
                float(row.get("velocity_24h", 0.0) or 0.0),
                float(row.get("dormancy_days", 0.0) or 0.0),
                float(row["score"]),
                str(row["category"]),
                now,
            ])
        cols = [
            "wallet", "tx_count", "first_seen", "last_seen", "unique_counterparties",
            "avg_tx_amount", "median_tx_amount", "herfindahl_index", "smart_contract_ratio",
            "velocity_per_hour", "dormancy_days", "score", "category", "updated_at",
        ]
        self.client.insert("wallets.wallet_stats", rows, column_names=cols)

    def write_score_history(self, scores: pd.DataFrame) -> None:
        if scores.empty:
            return
        now = datetime.utcnow()
        rows = [[str(w), float(r["score"]), str(r["category"]), now] for w, r in scores.iterrows()]
        self.client.insert("wallets.score_history", rows,
                           column_names=["wallet", "score", "category", "ts"])

    def write_token_scores(self, scores: pd.DataFrame) -> None:
        if scores.empty:
            return
        now = datetime.utcnow()
        rows = [
            [
                str(r["mint"]), str(r["category"]),
                float(r["confidence"]), float(r["risk_score"]),
                int(r["holders"]), float(r["volume_24h"]), now,
            ]
            for _, r in scores.iterrows()
        ]
        self.client.insert("wallets.token_scores", rows,
                           column_names=["mint", "category", "confidence", "risk_score",
                                         "holders", "volume_24h", "updated_at"])
