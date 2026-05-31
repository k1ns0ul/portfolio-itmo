from __future__ import annotations

import logging
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path

import joblib
import numpy as np
import pandas as pd
import xgboost as xgb

log = logging.getLogger(__name__)


FEATURE_COLUMNS: list[str] = [
    "collaborative_score",
    "promo_discount",
    "promo_popularity",
    "promo_freshness",
    "category_match_score",
    "time_match",
    "partner_diversity_penalty",
]


@dataclass
class RankCandidate:
    promo_id: int
    partner_id: int
    category_id: int | None
    discount: float
    discount_type: str
    popularity: float
    age_days: float
    collaborative_score: float


@dataclass
class RankerParams:
    n_estimators: int = 200
    max_depth: int = 5
    learning_rate: float = 0.1
    objective: str = "rank:pairwise"
    random_state: int = 42


class ContextualRanker:
    def __init__(self, params: RankerParams | None = None) -> None:
        self.params = params or RankerParams()
        self.model: xgb.XGBRanker | None = None

    def fit(self, features_df: pd.DataFrame, labels: np.ndarray, group_sizes: list[int]) -> dict[str, float]:
        if features_df.empty:
            raise ValueError("empty features for ranker")
        model = xgb.XGBRanker(
            objective=self.params.objective,
            n_estimators=self.params.n_estimators,
            max_depth=self.params.max_depth,
            learning_rate=self.params.learning_rate,
            random_state=self.params.random_state,
            tree_method="hist",
        )
        model.fit(features_df[FEATURE_COLUMNS], labels, group=group_sizes)
        self.model = model
        return {"trained_groups": len(group_sizes), "rows": len(features_df)}

    def rank(
        self,
        user_profile: dict,
        candidates: list[RankCandidate],
        already_picked_partners: set[int] | None = None,
    ) -> list[tuple[RankCandidate, float, str]]:
        if not candidates:
            return []
        already_picked = already_picked_partners or set()
        rows = []
        for c in candidates:
            rows.append(self._featurize(user_profile, c, already_picked))
        df = pd.DataFrame(rows, columns=FEATURE_COLUMNS)

        if self.model is None:
            scores = self._heuristic_scores(df)
        else:
            scores = self.model.predict(df)

        sorted_idx = np.argsort(-scores)
        out: list[tuple[RankCandidate, float, str]] = []
        seen_partners: set[int] = set()
        for i in sorted_idx:
            cand = candidates[int(i)]
            reason = self._reason(df.iloc[int(i)], cand, user_profile)
            seen_partners.add(cand.partner_id)
            out.append((cand, float(scores[int(i)]), reason))
        return out

    def _featurize(
        self,
        user_profile: dict,
        cand: RankCandidate,
        already_picked: set[int],
    ) -> dict:
        favorites = user_profile.get("favorite_categories", []) or []
        cat = cand.category_id
        if cat is not None and cat in favorites[:3]:
            cat_match = 1.0
        elif cat is not None and cat in favorites:
            cat_match = 0.5
        else:
            cat_match = 0.0

        time_pref = user_profile.get("time_preference", "afternoon")
        now_bucket = _bucket(datetime.now(timezone.utc).hour)
        time_match = 1.0 if time_pref == now_bucket else 0.0

        diversity_penalty = 0.0
        if cand.partner_id in already_picked:
            diversity_penalty = 1.0

        freshness = 1.0 / (1.0 + max(0.0, cand.age_days))

        return {
            "collaborative_score": cand.collaborative_score,
            "promo_discount": float(cand.discount),
            "promo_popularity": float(cand.popularity),
            "promo_freshness": float(freshness),
            "category_match_score": cat_match,
            "time_match": time_match,
            "partner_diversity_penalty": diversity_penalty,
        }

    def _heuristic_scores(self, df: pd.DataFrame) -> np.ndarray:
        weights = np.array([0.45, 0.10, 0.10, 0.10, 0.15, 0.05, -0.15], dtype=np.float64)
        normed = df.copy()
        for col in ("promo_discount", "promo_popularity"):
            mx = max(1e-9, normed[col].max())
            normed[col] = normed[col] / mx
        return (normed[FEATURE_COLUMNS].to_numpy() * weights).sum(axis=1)

    def _reason(self, feature_row: pd.Series, cand: RankCandidate, profile: dict) -> str:
        if feature_row["category_match_score"] >= 1.0:
            return "Любимая категория"
        if feature_row["collaborative_score"] > 0.5:
            return "Популярно у похожих пользователей"
        if feature_row["promo_freshness"] > 0.7:
            return "Свежее предложение"
        if feature_row["promo_discount"] > 0:
            return f"Скидка {cand.discount:g}"
        return "Подобрано для вас"

    def save(self, path: str | Path) -> None:
        path = Path(path)
        path.parent.mkdir(parents=True, exist_ok=True)
        joblib.dump({"model": self.model, "params": self.params}, path)

    def load(self, path: str | Path) -> None:
        blob = joblib.load(path)
        self.model = blob.get("model")
        self.params = blob.get("params", self.params)


def _bucket(hour: int) -> str:
    if 6 <= hour < 12:
        return "morning"
    if 12 <= hour < 18:
        return "afternoon"
    if 18 <= hour < 24:
        return "evening"
    return "night"
