from __future__ import annotations

import logging

import numpy as np
import pandas as pd

log = logging.getLogger(__name__)


LABEL_COLUMNS: list[str] = ["target_score", "target_label"]

LABEL_LEGIT = 0
LABEL_SUSPICIOUS = 1
LABEL_SCAM = 2


class HeuristicLabeler:
    def __init__(
        self,
        scam_threshold: float = 0.5,
        suspicious_threshold: float = 0.4,
    ) -> None:
        self.scam_threshold = scam_threshold
        self.suspicious_threshold = suspicious_threshold

    def label(self, features: pd.DataFrame, raw: pd.DataFrame) -> pd.DataFrame:
        if features.empty:
            return pd.DataFrame(columns=LABEL_COLUMNS)

        scam = self._scam_score(features, raw)
        susp = self._suspicious_score(features)
        legit = np.clip(1.0 - scam - susp, 0.0, 1.0)

        score_table = np.vstack([legit, susp, scam]).T
        labels = np.argmax(score_table, axis=1)

        target_score = (
            100.0 * legit
            + 50.0 * susp
            + 10.0 * scam
        )
        target_score = np.clip(target_score, 0.0, 100.0)

        labels = self._refine_labels(labels, scam, susp)

        out = pd.DataFrame(
            {
                "target_score": target_score,
                "target_label": labels,
                "scam_signal": scam,
                "suspicious_signal": susp,
            },
            index=features.index,
        )
        return out

    def _scam_score(self, f: pd.DataFrame, raw: pd.DataFrame) -> np.ndarray:
        wash = self._wash_trading_share(f.index, raw)
        pump_dump = self._pump_dump_signal(f.index, raw)

        young = f["wallet_age_days"].le(2).astype(float)
        high_freq = (f["tx_count"] / f["wallet_age_days"].clip(lower=1)).clip(0, 200) / 200.0

        score = 0.4 * wash + 0.3 * pump_dump + 0.15 * young + 0.15 * high_freq
        return np.clip(score.to_numpy(), 0.0, 1.0)

    def _suspicious_score(self, f: pd.DataFrame) -> np.ndarray:
        night = (f["night_ratio"] - 0.4).clip(lower=0) / 0.6
        rounds = (f["round_amount_ratio"] - 0.6).clip(lower=0) / 0.4
        velocity = (f["velocity_24h"] - 3.0).clip(lower=0) / 10.0
        velocity = velocity.clip(upper=1.0)

        herf = (f["herfindahl_index"] - 0.5).clip(lower=0) / 0.5

        score = 0.3 * night + 0.25 * rounds + 0.25 * velocity + 0.2 * herf
        return np.clip(score.to_numpy(), 0.0, 1.0)

    def _wash_trading_share(self, wallets: pd.Index, raw: pd.DataFrame) -> pd.Series:
        if raw.empty:
            return pd.Series(0.0, index=wallets)
        df = raw[raw["sender"].isin(wallets)]
        if df.empty:
            return pd.Series(0.0, index=wallets)
        counts = df.groupby(["sender", "receiver"]).size().reset_index(name="cnt")
        per_wallet_total = counts.groupby("sender")["cnt"].sum()
        top3 = counts.sort_values("cnt", ascending=False).groupby("sender").head(3)
        top3_total = top3.groupby("sender")["cnt"].sum()
        ratio = (top3_total / per_wallet_total).fillna(0.0)
        return ratio.reindex(wallets, fill_value=0.0)

    def _pump_dump_signal(self, wallets: pd.Index, raw: pd.DataFrame) -> pd.Series:
        if raw.empty:
            return pd.Series(0.0, index=wallets)
        df = raw[raw["sender"].isin(wallets)].copy()
        if df.empty:
            return pd.Series(0.0, index=wallets)
        df["block_time"] = pd.to_datetime(df["block_time"], utc=True)
        df["hour"] = df["block_time"].dt.floor("H")
        hourly = df.groupby(["sender", "hour"])["amount"].sum().reset_index()
        signals: dict[str, float] = {}
        for wallet, w in hourly.groupby("sender"):
            volumes = w["amount"].to_numpy().astype("float64")
            if volumes.size < 3:
                signals[wallet] = 0.0
                continue
            peak_idx = int(np.argmax(volumes))
            if peak_idx == 0 or peak_idx == len(volumes) - 1:
                signals[wallet] = 0.0
                continue
            peak = volumes[peak_idx]
            before = volumes[max(0, peak_idx - 3):peak_idx].mean()
            after = volumes[peak_idx + 1:peak_idx + 4].mean()
            if peak <= 0 or before <= 0:
                signals[wallet] = 0.0
                continue
            spike = (peak - before) / (before + 1e-9)
            drop = (peak - after) / (peak + 1e-9)
            signals[wallet] = float(np.clip(min(spike, 5.0) / 5.0 * drop, 0.0, 1.0))
        return pd.Series(signals).reindex(wallets, fill_value=0.0)

    def _refine_labels(self, labels: np.ndarray, scam: np.ndarray, susp: np.ndarray) -> np.ndarray:
        out = labels.copy()
        out[scam >= self.scam_threshold] = LABEL_SCAM
        only_susp = (scam < self.scam_threshold) & (susp >= self.suspicious_threshold)
        out[only_susp] = LABEL_SUSPICIOUS
        return out.astype(int)
