from __future__ import annotations

import logging
from dataclasses import dataclass
from datetime import datetime, timezone

import numpy as np
import pandas as pd

log = logging.getLogger(__name__)


@dataclass
class UserFeatures:
    user_id: int
    purchase_count: int
    total_spent: float
    avg_check: float
    favorite_categories: list[int]
    days_since_last_purchase: float
    purchase_frequency: float
    promo_activation_rate: float
    time_preference: str
    referral_count: int


TIME_BUCKETS = ["morning", "afternoon", "evening", "night"]


def _time_bucket(hour: int) -> str:
    if 6 <= hour < 12:
        return "morning"
    if 12 <= hour < 18:
        return "afternoon"
    if 18 <= hour < 24:
        return "evening"
    return "night"


class UserFeatureSet:
    def __init__(self, purchases: pd.DataFrame, referral_edges: list[tuple[int, int]] | None = None) -> None:
        self.purchases = purchases
        self.referral_edges = referral_edges or []

    def build(self) -> pd.DataFrame:
        if self.purchases.empty:
            return pd.DataFrame()

        df = self.purchases.copy()
        df["created_at"] = pd.to_datetime(df["created_at"], utc=True)
        df["amount"] = df["amount"].astype("float64")
        df["hour"] = df["created_at"].dt.hour
        df["time_bucket"] = df["hour"].apply(_time_bucket)

        now = pd.Timestamp.now(tz=timezone.utc)

        ref_counts = self._referral_counts(df["user_id"].unique())

        records: list[dict] = []
        for user_id, group in df.groupby("user_id", sort=False):
            records.append(self._features_for(int(user_id), group, now, ref_counts.get(int(user_id), 0)))

        out = pd.DataFrame(records).set_index("user_id")
        log.info("built features for %d users", len(out))
        return out

    def features_as_dataclass(self, user_id: int) -> UserFeatures | None:
        df = self.build()
        if df.empty or user_id not in df.index:
            return None
        row = df.loc[user_id]
        return UserFeatures(
            user_id=user_id,
            purchase_count=int(row["purchase_count"]),
            total_spent=float(row["total_spent"]),
            avg_check=float(row["avg_check"]),
            favorite_categories=list(row["favorite_categories"]),
            days_since_last_purchase=float(row["days_since_last_purchase"]),
            purchase_frequency=float(row["purchase_frequency"]),
            promo_activation_rate=float(row["promo_activation_rate"]),
            time_preference=str(row["time_preference"]),
            referral_count=int(row["referral_count"]),
        )

    def _features_for(
        self,
        user_id: int,
        group: pd.DataFrame,
        now: pd.Timestamp,
        referral_count: int,
    ) -> dict:
        amounts = group["amount"].to_numpy()
        first_seen = group["created_at"].min()
        last_seen = group["created_at"].max()

        if "category_id" in group.columns:
            cat_series = group["category_id"].dropna().astype("int64")
            cat_counts = cat_series.value_counts()
            favorites = cat_counts.head(3).index.tolist()
        else:
            favorites = []

        active_days = max(1.0, (now - first_seen).total_seconds() / 86400.0)
        frequency = float(len(group)) / (active_days / 7.0)

        confirmed = (group["status"] == "confirmed").sum() if "status" in group.columns else len(group)
        activation_rate = float(confirmed) / max(1, len(group))

        bucket_counts = group["time_bucket"].value_counts()
        time_preference = str(bucket_counts.idxmax()) if not bucket_counts.empty else "afternoon"

        return {
            "user_id": user_id,
            "purchase_count": int(len(group)),
            "total_spent": float(amounts.sum()),
            "avg_check": float(amounts.mean()) if amounts.size else 0.0,
            "favorite_categories": [int(c) for c in favorites],
            "days_since_last_purchase": float((now - last_seen).total_seconds() / 86400.0),
            "purchase_frequency": frequency,
            "promo_activation_rate": activation_rate,
            "time_preference": time_preference,
            "referral_count": int(referral_count),
        }

    def _referral_counts(self, user_ids: np.ndarray) -> dict[int, int]:
        if not self.referral_edges:
            return {}
        counts: dict[int, int] = {}
        for referrer, _ in self.referral_edges:
            counts[int(referrer)] = counts.get(int(referrer), 0) + 1
        return counts
