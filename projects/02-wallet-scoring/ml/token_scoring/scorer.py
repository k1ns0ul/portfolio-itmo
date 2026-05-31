from __future__ import annotations

import logging
from datetime import datetime, timezone
from typing import Iterable

import numpy as np
import pandas as pd

from config import MLConfig
from db import ClickHouseDB

log = logging.getLogger(__name__)


_CAT_SCAM = "scam"
_CAT_SUSPICIOUS = "suspicious"
_CAT_LEGIT = "legit"


class TokenScorer:
    def __init__(self, cfg: MLConfig, db: ClickHouseDB | None = None) -> None:
        self.cfg = cfg
        self.db = db or ClickHouseDB(cfg)

    def close(self) -> None:
        self.db.close()

    def score_all(self) -> int:
        mints = self._list_tokens()
        log.info("scoring %d tokens", len(mints))
        scored = self.score_tokens(mints)
        self.db.write_token_scores(scored)
        return len(scored)

    def score_tokens(self, mints: Iterable[str]) -> pd.DataFrame:
        mints = [m for m in mints if m]
        if not mints:
            return pd.DataFrame()

        holders = self._fetch_token_holdings(mints)
        ages = self._fetch_token_ages(mints)
        volumes = self._fetch_token_volumes(mints)
        wallet_categories = self._fetch_wallet_categories(holders["holder"].unique().tolist())

        records: list[dict] = []
        for mint, group in holders.groupby("mint"):
            age_days = float(ages.get(mint, 0.0))
            volume_24h = float(volumes.get(mint, 0.0))

            holdings = group["amount"].to_numpy(dtype="float64")
            holdings = holdings[holdings > 0]
            if holdings.size == 0:
                continue

            gini = self._gini(holdings)
            n_holders = int(holdings.size)

            cats = wallet_categories.reindex(group["holder"]).fillna(_CAT_LEGIT)
            scam_share = float((cats == _CAT_SCAM).mean())
            susp_share = float((cats == _CAT_SUSPICIOUS).mean())
            bad_share = scam_share + susp_share

            category, confidence, risk = self._classify(gini, age_days, bad_share, scam_share)

            records.append({
                "mint": mint,
                "category": category,
                "confidence": confidence,
                "risk_score": risk,
                "holders": n_holders,
                "volume_24h": volume_24h,
                "gini": gini,
                "bad_holder_share": bad_share,
                "age_days": age_days,
            })

        df = pd.DataFrame.from_records(records)
        log.info("scored %d tokens", len(df))
        return df

    def _list_tokens(self) -> list[str]:
        rows = self.db.client.query(
            "SELECT DISTINCT program_id AS mint FROM wallets.transactions "
            "WHERE program_id != '' LIMIT 50000"
        ).result_rows
        return [r[0] for r in rows]

    def _fetch_token_holdings(self, mints: list[str]) -> pd.DataFrame:
        q = """
            SELECT program_id AS mint,
                   sender AS holder,
                   sum(amount) AS amount
            FROM wallets.transactions
            WHERE program_id IN {mints:Array(String)}
            GROUP BY program_id, sender
        """
        return self.db.client.query_df(q, parameters={"mints": mints})

    def _fetch_token_ages(self, mints: list[str]) -> pd.Series:
        q = """
            SELECT program_id AS mint,
                   dateDiff('day', min(block_time), now()) AS age_days
            FROM wallets.transactions
            WHERE program_id IN {mints:Array(String)}
            GROUP BY program_id
        """
        df = self.db.client.query_df(q, parameters={"mints": mints})
        if df.empty:
            return pd.Series(dtype="float64")
        return df.set_index("mint")["age_days"].astype("float64")

    def _fetch_token_volumes(self, mints: list[str]) -> pd.Series:
        q = """
            SELECT program_id AS mint, sum(amount) AS vol
            FROM wallets.transactions
            WHERE program_id IN {mints:Array(String)}
              AND block_time >= now() - INTERVAL 1 DAY
            GROUP BY program_id
        """
        df = self.db.client.query_df(q, parameters={"mints": mints})
        if df.empty:
            return pd.Series(dtype="float64")
        return df.set_index("mint")["vol"].astype("float64")

    def _fetch_wallet_categories(self, wallets: list[str]) -> pd.Series:
        if not wallets:
            return pd.Series(dtype="object")
        q = """
            SELECT wallet, category FROM wallets.wallet_stats FINAL
            WHERE wallet IN {wallets:Array(String)}
        """
        df = self.db.client.query_df(q, parameters={"wallets": wallets})
        if df.empty:
            return pd.Series(dtype="object")
        return df.set_index("wallet")["category"].astype(str)

    def _gini(self, x: np.ndarray) -> float:
        if x.size == 0 or x.sum() == 0:
            return 0.0
        sorted_x = np.sort(x)
        n = sorted_x.size
        cum = np.cumsum(sorted_x)
        return float((2.0 * np.sum((np.arange(1, n + 1)) * sorted_x) - (n + 1) * cum[-1]) / (n * cum[-1]))

    def _classify(self, gini: float, age_days: float, bad_share: float, scam_share: float) -> tuple[str, float, float]:
        if gini > 0.9 and age_days < 7 and bad_share > 0.5:
            return _CAT_SCAM, min(0.99, 0.6 + 0.4 * bad_share), float(np.clip(gini * 100, 60, 100))
        if gini > 0.7 or bad_share > 0.3:
            return _CAT_SUSPICIOUS, min(0.95, 0.4 + 0.5 * bad_share), float(np.clip(50 + gini * 30, 40, 80))
        return _CAT_LEGIT, 0.5 + (1.0 - gini) * 0.4, float(np.clip(20 + gini * 30, 0, 50))
