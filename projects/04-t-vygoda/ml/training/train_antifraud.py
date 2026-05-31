from __future__ import annotations

import asyncio
import json
import logging
from pathlib import Path

import numpy as np

from antifraud.features import FraudFeatures
from antifraud.graph import ReferralGraph
from antifraud.model import DetectorParams, FraudDetector
from config import MLConfig
from db import PostgresDB

log = logging.getLogger(__name__)


async def run(cfg: MLConfig) -> dict:
    db = PostgresDB(cfg)
    await db.connect()
    try:
        edges = await db.fetch_referral_edges()
        bonuses = await db.fetch_referral_bonuses(since_days=90)

        graph = ReferralGraph(edges, bonuses=bonuses)
        purchases_by_user = await _purchases_count(db, edges)

        features = FraudFeatures(
            graph=graph.graph,
            bonuses=bonuses,
            purchases_by_user=purchases_by_user,
        ).build()
        if features.empty:
            raise RuntimeError("no features to train antifraud")

        detector = FraudDetector(params=DetectorParams(
            contamination=cfg.fraud_contamination,
            random_state=cfg.random_state,
        ))
        detector.fit(features)

        out = Path(cfg.model_dir)
        out.mkdir(parents=True, exist_ok=True)
        detector.save(out)

        preds = detector.predict(features)
        flagged = [p for p in preds if p.is_fraud]
        flagged.sort(key=lambda p: -p.score)
        top = [
            {"user_id": p.user_id, "score": round(p.score, 4), "reasons": p.reasons}
            for p in flagged[:10]
        ]

        metrics = {
            "total_users": int(len(features)),
            "anomalies": int(len(flagged)),
            "contamination": cfg.fraud_contamination,
            "top_10": top,
        }
        (out / "antifraud_metrics.json").write_text(
            json.dumps(metrics, indent=2, ensure_ascii=False),
            encoding="utf-8",
        )
        log.info("antifraud trained: %d/%d anomalies", len(flagged), len(features))
        return metrics
    finally:
        await db.close()


async def _purchases_count(db: PostgresDB, edges: list[tuple[int, int]]) -> dict[int, int]:
    if not edges:
        return {}
    user_ids = list({uid for _, uid in edges})
    rows = await db.pool.fetch(
        "SELECT user_id, COUNT(*) AS n FROM purchases WHERE user_id = ANY($1::bigint[]) GROUP BY user_id",
        user_ids,
    )
    return {int(r["user_id"]): int(r["n"]) for r in rows}


def main() -> None:
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s: %(message)s")
    cfg = MLConfig()
    asyncio.run(run(cfg))


if __name__ == "__main__":
    main()
