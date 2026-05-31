from __future__ import annotations

import logging
from datetime import datetime, timezone
from typing import Iterable

import clickhouse_connect
import pandas as pd
from clickhouse_connect.driver.client import Client

from config import MLConfig

log = logging.getLogger(__name__)


FEATURE_COLUMNS: list[str] = [
    "ofi",
    "vpin",
    "price_impact",
    "avg_swap_size",
    "buy_ratio",
    "cumulative_volume",
    "price_range",
    "price_close",
    "swap_count",
]


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

    def fetch_features(self, pair: str | None = None, interval_sec: int = 60, limit: int = 50000) -> pd.DataFrame:
        q = """
            SELECT pair, interval_sec, window_start, window_end,
                   ofi, vpin, price_impact, avg_swap_size, buy_ratio,
                   cumulative_volume, price_range, price_open, price_close, swap_count
            FROM orderflow.feature_windows
            WHERE interval_sec = {interval:UInt32}
        """
        params: dict[str, object] = {"interval": interval_sec, "limit": limit}
        if pair:
            q += " AND pair = {pair:String}"
            params["pair"] = pair
        q += " ORDER BY pair, window_end ASC LIMIT {limit:UInt64}"
        df = self.client.query_df(q, parameters=params)
        log.info("fetched %d feature rows", len(df))
        return df

    def fetch_latest(self, pair: str, interval_sec: int, n: int) -> pd.DataFrame:
        q = """
            SELECT pair, interval_sec, window_start, window_end,
                   ofi, vpin, price_impact, avg_swap_size, buy_ratio,
                   cumulative_volume, price_range, price_open, price_close, swap_count
            FROM orderflow.feature_windows
            WHERE pair = {pair:String} AND interval_sec = {interval:UInt32}
            ORDER BY window_end DESC LIMIT {n:UInt32}
        """
        df = self.client.query_df(q, parameters={"pair": pair, "interval": interval_sec, "n": n})
        return df.iloc[::-1].reset_index(drop=True)

    def list_pairs(self, interval_sec: int) -> list[str]:
        q = """
            SELECT DISTINCT pair FROM orderflow.feature_windows
            WHERE interval_sec = {interval:UInt32}
        """
        rows = self.client.query(q, parameters={"interval": interval_sec}).result_rows
        return [r[0] for r in rows]

    def write_predictions(self, predictions: Iterable[dict]) -> int:
        rows = []
        now = datetime.now(timezone.utc)
        cols = ["pair", "window_end", "direction", "confidence", "xgb_prob", "lstm_prob", "created_at"]
        for p in predictions:
            rows.append([
                str(p["pair"]),
                p["window_end"],
                str(p["direction"]),
                float(p["confidence"]),
                float(p["xgb_prob"]),
                float(p["lstm_prob"]),
                now,
            ])
        if not rows:
            return 0
        self.client.insert("orderflow.predictions", rows, column_names=cols)
        return len(rows)
