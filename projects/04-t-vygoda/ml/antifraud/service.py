from __future__ import annotations

import logging
from dataclasses import asdict, dataclass, field
from datetime import datetime, timezone
from pathlib import Path

import pandas as pd

from antifraud.features import FraudFeatures
from antifraud.graph import AuditReport, ReferralGraph
from antifraud.model import DetectorParams, FraudDetector, FraudPrediction
from config import MLConfig
from db import PostgresDB

log = logging.getLogger(__name__)


@dataclass
class FullAuditReport:
    generated_at: datetime
    total_users: int
    flagged_count: int
    rings: list[list[int]] = field(default_factory=list)
    dense_clusters: list[list[int]] = field(default_factory=list)
    velocity_abusers: list[int] = field(default_factory=list)
    bonus_outliers: list[int] = field(default_factory=list)
    top_suspicious: list[dict] = field(default_factory=list)

    def to_dict(self) -> dict:
        d = asdict(self)
        d["generated_at"] = self.generated_at.isoformat()
        return d


class AntifraudService:
    def __init__(self, cfg: MLConfig, db: PostgresDB | None = None) -> None:
        self.cfg = cfg
        self.db = db
        self.detector = FraudDetector(params=DetectorParams(
            contamination=cfg.fraud_contamination,
            random_state=cfg.random_state,
        ))

    async def load(self) -> bool:
        path = Path(self.cfg.model_dir)
        if not (path / "fraud_iso.joblib").exists():
            log.warning("fraud model not found in %s", path)
            return False
        self.detector.load(path)
        return True

    async def run_audit(self) -> FullAuditReport:
        if self.db is None:
            raise RuntimeError("db is not configured")
        edges = await self.db.fetch_referral_edges()
        bonuses = await self.db.fetch_referral_bonuses(since_days=30)

        graph = ReferralGraph(edges, bonuses=bonuses)
        base = graph.full_scan(
            ring_max_length=self.cfg.fraud_ring_max_length,
            velocity_threshold=self.cfg.fraud_velocity_threshold,
            window_hours=self.cfg.fraud_window_hours,
        )

        purchases_by_user = await self._purchases_per_referral(edges)
        features = FraudFeatures(
            graph=graph.graph,
            bonuses=bonuses,
            purchases_by_user=purchases_by_user,
        ).build()

        flagged: list[FraudPrediction] = []
        if not features.empty:
            if self.detector.iso is None:
                self.detector.fit(features)
            flagged = self.detector.predict(features)

        top_suspicious = sorted(flagged, key=lambda p: -p.score)[:50]
        top_dicts = [
            {
                "user_id": p.user_id,
                "score": p.score,
                "is_fraud": p.is_fraud,
                "reasons": p.reasons,
            }
            for p in top_suspicious if p.is_fraud
        ]

        return FullAuditReport(
            generated_at=datetime.now(timezone.utc),
            total_users=base.total_users,
            flagged_count=sum(1 for p in flagged if p.is_fraud),
            rings=base.rings,
            dense_clusters=base.dense_clusters,
            velocity_abusers=base.velocity_abusers,
            bonus_outliers=base.bonus_outliers,
            top_suspicious=top_dicts,
        )

    async def retrain(self) -> dict:
        if self.db is None:
            raise RuntimeError("db is not configured")
        edges = await self.db.fetch_referral_edges()
        bonuses = await self.db.fetch_referral_bonuses(since_days=90)
        graph = ReferralGraph(edges, bonuses=bonuses)
        purchases_by_user = await self._purchases_per_referral(edges)
        features = FraudFeatures(
            graph=graph.graph,
            bonuses=bonuses,
            purchases_by_user=purchases_by_user,
        ).build()
        if features.empty:
            raise RuntimeError("no features to train on")
        metrics = self.detector.fit(features)
        self.detector.save(self.cfg.model_dir)
        return {"users": len(features), "metrics": metrics}

    async def _purchases_per_referral(self, edges: list[tuple[int, int]]) -> dict[int, int]:
        if not edges:
            return {}
        user_ids = list({uid for _, uid in edges})
        if not user_ids:
            return {}
        q = """
            SELECT user_id, COUNT(*) AS n
            FROM purchases WHERE user_id = ANY($1::bigint[])
            GROUP BY user_id
        """
        rows = await self.db.pool.fetch(q, user_ids)
        return {int(r["user_id"]): int(r["n"]) for r in rows}
