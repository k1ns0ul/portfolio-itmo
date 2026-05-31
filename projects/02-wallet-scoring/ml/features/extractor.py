from __future__ import annotations

import logging
from datetime import datetime, timezone

import numpy as np
import pandas as pd

log = logging.getLogger(__name__)


FEATURE_COLUMNS: list[str] = [
    "wallet_age_days",
    "tx_count",
    "tx_count_7d",
    "unique_counterparties",
    "avg_tx_amount",
    "median_tx_amount",
    "max_tx_amount",
    "herfindahl_index",
    "smart_contract_ratio",
    "velocity_24h",
    "dormancy_days",
    "incoming_ratio",
    "unique_programs",
    "avg_time_between_tx",
    "night_ratio",
    "round_amount_ratio",
]


_SC_NON_PROGRAMS = {
    "11111111111111111111111111111111",
    "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA",
    "TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb",
    "ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL",
    "MemoSq4gqABAXKb96qnH8TysNcWxMyWCqXgDLGmfcHr",
    "ComputeBudget111111111111111111111111111111",
}


class FeatureExtractor:
    def __init__(self, lamports_norm: float = 1e9) -> None:
        self.lamports_norm = lamports_norm

    def extract(self, df: pd.DataFrame) -> pd.DataFrame:
        if df.empty:
            return pd.DataFrame(columns=FEATURE_COLUMNS)

        df = self._prepare(df)
        now = pd.Timestamp.now(tz=timezone.utc)
        cutoff_7d = now - pd.Timedelta(days=7)
        cutoff_24h = now - pd.Timedelta(hours=24)

        records: list[dict[str, float]] = []
        for wallet, group in df.groupby("wallet_address", sort=False):
            features = self._wallet_features(wallet, group, now, cutoff_7d, cutoff_24h)
            records.append(features)

        out = pd.DataFrame(records).set_index("wallet_address")
        out = out[FEATURE_COLUMNS].astype(float).fillna(0.0)
        log.info("extracted features for %d wallets", len(out))
        return out

    def _prepare(self, df: pd.DataFrame) -> pd.DataFrame:
        df = df.copy()
        df["block_time"] = pd.to_datetime(df["block_time"], utc=True)
        df["amount_norm"] = df["amount"].astype("float64") / self.lamports_norm

        sender = df[["sender", "block_time", "amount_norm", "program_id", "receiver"]].rename(
            columns={"sender": "wallet_address", "receiver": "counterparty"}
        )
        sender["direction"] = "out"
        receiver = df[["receiver", "block_time", "amount_norm", "program_id", "sender"]].rename(
            columns={"receiver": "wallet_address", "sender": "counterparty"}
        )
        receiver["direction"] = "in"

        joined = pd.concat([sender, receiver], ignore_index=True)
        joined = joined[joined["wallet_address"].notna() & (joined["wallet_address"] != "")]
        joined = joined.sort_values(["wallet_address", "block_time"], kind="stable")
        return joined

    def _wallet_features(
        self,
        wallet: str,
        group: pd.DataFrame,
        now: pd.Timestamp,
        cutoff_7d: pd.Timestamp,
        cutoff_24h: pd.Timestamp,
    ) -> dict[str, float]:
        amounts = group["amount_norm"].to_numpy()
        first_seen = group["block_time"].iloc[0]
        last_seen = group["block_time"].iloc[-1]

        cps = group["counterparty"].fillna("").to_numpy()
        cps = cps[cps != ""]
        cp_totals = pd.Series(cps).value_counts()

        herf = float((cp_totals / cp_totals.sum()).pow(2).sum()) if len(cp_totals) else 0.0

        sc_mask = ~group["program_id"].isin(_SC_NON_PROGRAMS)
        sc_ratio = float(sc_mask.mean()) if len(group) else 0.0

        in_mask = group["direction"] == "in"
        incoming_ratio = float(in_mask.mean()) if len(group) else 0.0

        unique_programs = int(group["program_id"].nunique())

        recent_24h = group[group["block_time"] >= cutoff_24h]
        velocity = float(len(recent_24h) / 24.0)

        tx_7d = int((group["block_time"] >= cutoff_7d).sum())

        dormancy = float((now - last_seen).total_seconds() / 86400.0)
        if dormancy < 0:
            dormancy = 0.0

        if len(group) > 1:
            deltas = group["block_time"].diff().dt.total_seconds().dropna()
            avg_gap = float(deltas.mean())
        else:
            avg_gap = 0.0

        hours = group["block_time"].dt.hour
        night = float(((hours >= 0) & (hours < 6)).mean()) if len(group) else 0.0

        round_mask = self._round_amount_mask(amounts)
        round_ratio = float(round_mask.mean()) if amounts.size else 0.0

        return {
            "wallet_address": wallet,
            "wallet_age_days": float((now - first_seen).total_seconds() / 86400.0),
            "tx_count": float(len(group)),
            "tx_count_7d": float(tx_7d),
            "unique_counterparties": float(cp_totals.size),
            "avg_tx_amount": float(amounts.mean()) if amounts.size else 0.0,
            "median_tx_amount": float(np.median(amounts)) if amounts.size else 0.0,
            "max_tx_amount": float(amounts.max()) if amounts.size else 0.0,
            "herfindahl_index": herf,
            "smart_contract_ratio": sc_ratio,
            "velocity_24h": velocity,
            "dormancy_days": dormancy,
            "incoming_ratio": incoming_ratio,
            "unique_programs": float(unique_programs),
            "avg_time_between_tx": avg_gap,
            "night_ratio": night,
            "round_amount_ratio": round_ratio,
        }

    def _round_amount_mask(self, amounts: np.ndarray) -> np.ndarray:
        if amounts.size == 0:
            return np.zeros(0, dtype=bool)
        scaled = amounts * 1e3
        nearest = np.round(scaled / 100.0) * 100.0
        diff = np.abs(scaled - nearest)
        return diff < 1e-6
