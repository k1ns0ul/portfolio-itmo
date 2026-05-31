from __future__ import annotations

import logging
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path

import numpy as np
import pandas as pd

from config import MLConfig
from db import PostgresDB
from recommender.collaborative import ALSParams, CollaborativeFilter
from recommender.contextual import ContextualRanker, RankCandidate
from recommender.features import UserFeatureSet

log = logging.getLogger(__name__)


@dataclass
class Recommendation:
    user_id: int
    promo_id: int
    score: float
    reason: str
    generated_at: datetime = field(default_factory=lambda: datetime.now(timezone.utc))


class RecommenderService:
    def __init__(self, cfg: MLConfig, db: PostgresDB | None = None) -> None:
        self.cfg = cfg
        self.db = db
        self.collab = CollaborativeFilter(params=ALSParams(
            n_factors=cfg.als_factors,
            n_iterations=cfg.als_iters,
            regularization=cfg.als_reg,
            alpha=cfg.als_alpha,
            random_state=cfg.random_state,
        ))
        self.ranker = ContextualRanker()
        self.user_features: pd.DataFrame = pd.DataFrame()

    async def load(self) -> bool:
        model_dir = Path(self.cfg.model_dir)
        collab_path = model_dir / "collab.npz"
        ranker_path = model_dir / "ranker.joblib"
        features_path = model_dir / "user_features.parquet"
        if not collab_path.exists() or not features_path.exists():
            log.warning("models not found in %s", model_dir)
            return False
        self.collab.load(collab_path)
        if ranker_path.exists():
            self.ranker.load(ranker_path)
        self.user_features = pd.read_parquet(features_path)
        log.info("recommender loaded: users=%d, ranker=%s",
                 len(self.user_features), "trained" if self.ranker.model else "heuristic")
        return True

    async def get_recommendations(self, user_id: int, limit: int = 10) -> list[Recommendation]:
        if self.db is None:
            return []
        profile = self.user_features.loc[user_id].to_dict() if user_id in self.user_features.index else {}
        promos = await self.db.fetch_promos_active()
        if promos.empty:
            return []

        top_categories = self.collab.recommend(user_id, n=self.cfg.refresh_top_n)
        category_scores = {int(cid): score for cid, score in top_categories}
        if category_scores:
            promos = promos[promos["category_id"].isin(category_scores.keys()) | promos["category_id"].isna()]
        if promos.empty:
            promos = await self.db.fetch_promos_active()

        candidates = self._to_candidates(promos, category_scores)
        ranked = self.ranker.rank(profile, candidates)

        recs: list[Recommendation] = []
        seen_partners: set[int] = set()
        for cand, score, reason in ranked:
            if cand.partner_id in seen_partners and len(recs) >= limit // 2:
                continue
            seen_partners.add(cand.partner_id)
            recs.append(Recommendation(
                user_id=user_id,
                promo_id=cand.promo_id,
                score=float(score),
                reason=reason,
            ))
            if len(recs) >= limit:
                break
        return recs

    async def refresh_all(self) -> int:
        if self.db is None:
            return 0
        users = await self.db.list_active_user_ids(days=30)
        log.info("refresh for %d active users", len(users))
        total = 0
        batch_size = self.cfg.refresh_batch_size
        for i in range(0, len(users), batch_size):
            batch = users[i:i + batch_size]
            for uid in batch:
                recs = await self.get_recommendations(uid, limit=self.cfg.refresh_top_n)
                payload = [{"promo_id": r.promo_id, "score": r.score, "reason": r.reason} for r in recs]
                await self.db.write_recommendations(uid, payload)
                total += len(payload)
            log.info("refreshed batch starting %d / %d", i, len(users))
        return total

    async def retrain(self) -> dict:
        if self.db is None:
            raise RuntimeError("db is not configured")
        purchases = await self.db.fetch_all_purchases(since_days=180)
        if purchases.empty:
            raise RuntimeError("no data to train")
        edges = await self.db.fetch_referral_edges()

        feature_builder = UserFeatureSet(purchases, edges)
        user_features_df = feature_builder.build()

        matrix, u_idx, i_idx = CollaborativeFilter.build_matrix(
            purchases.dropna(subset=["category_id"]),
            user_col="user_id",
            item_col="category_id",
            value_col="amount",
        )
        self.collab.fit(matrix, u_idx, i_idx)

        Path(self.cfg.model_dir).mkdir(parents=True, exist_ok=True)
        self.collab.save(Path(self.cfg.model_dir) / "collab.npz")
        user_features_df.to_parquet(Path(self.cfg.model_dir) / "user_features.parquet")
        self.user_features = user_features_df

        return {
            "users": len(u_idx),
            "categories": len(i_idx),
            "features_rows": len(user_features_df),
        }

    def _to_candidates(self, promos: pd.DataFrame, category_scores: dict[int, float]) -> list[RankCandidate]:
        now = pd.Timestamp.now(tz=timezone.utc)
        out: list[RankCandidate] = []
        for _, row in promos.iterrows():
            cat_id = int(row["category_id"]) if pd.notna(row["category_id"]) else None
            score = category_scores.get(cat_id, 0.0) if cat_id is not None else 0.0
            created_at = pd.Timestamp(row["created_at"]).tz_convert("UTC") if row.get("created_at") is not None else now
            age_days = float((now - created_at).total_seconds() / 86400.0)
            out.append(RankCandidate(
                promo_id=int(row["id"]),
                partner_id=int(row["partner_id"]),
                category_id=cat_id,
                discount=float(row["discount"]),
                discount_type=str(row["type"]),
                popularity=float(row.get("current_uses", 0) or 0),
                age_days=age_days,
                collaborative_score=score,
            ))
        return out
