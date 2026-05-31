from __future__ import annotations

import asyncio
import logging
from dataclasses import asdict
from pathlib import Path
from typing import Iterable

import numpy as np
import pandas as pd

from config import MLConfig
from db import PostgresDB
from recommender.collaborative import CollaborativeFilter
from recommender.features import UserFeatureSet

log = logging.getLogger(__name__)


async def run(cfg: MLConfig) -> dict:
    db = PostgresDB(cfg)
    await db.connect()
    try:
        purchases = await db.fetch_all_purchases(since_days=180)
        if purchases.empty:
            raise RuntimeError("no purchases for training")
        edges = await db.fetch_referral_edges()

        train_df, holdout_df = _split_holdout(purchases, ratio=0.1, random_state=cfg.random_state)

        matrix, u_idx, i_idx = CollaborativeFilter.build_matrix(
            train_df.dropna(subset=["category_id"]),
            user_col="user_id",
            item_col="category_id",
            value_col="amount",
        )

        collab = CollaborativeFilter()
        collab.params.n_factors = cfg.als_factors
        collab.params.n_iterations = cfg.als_iters
        collab.params.regularization = cfg.als_reg
        collab.params.alpha = cfg.als_alpha
        collab.fit(matrix, u_idx, i_idx)

        features = UserFeatureSet(purchases, edges).build()
        out = Path(cfg.model_dir)
        out.mkdir(parents=True, exist_ok=True)
        collab.save(out / "collab.npz")
        features.to_parquet(out / "user_features.parquet")

        metrics = _evaluate(collab, holdout_df, k=10)
        log.info("training metrics: %s", metrics)
        (out / "recommender_metrics.json").write_text(_metrics_to_json(metrics), encoding="utf-8")
        return metrics
    finally:
        await db.close()


def _split_holdout(df: pd.DataFrame, ratio: float, random_state: int) -> tuple[pd.DataFrame, pd.DataFrame]:
    rng = np.random.default_rng(random_state)
    mask = rng.random(len(df)) < ratio
    return df[~mask].reset_index(drop=True), df[mask].reset_index(drop=True)


def _evaluate(collab: CollaborativeFilter, holdout: pd.DataFrame, k: int = 10) -> dict:
    if holdout.empty:
        return {"precision_at_k": 0.0, "recall_at_k": 0.0, "ndcg_at_k": 0.0, "k": k}
    truth: dict[int, set[int]] = {}
    for user_id, group in holdout.dropna(subset=["category_id"]).groupby("user_id"):
        truth[int(user_id)] = set(group["category_id"].astype(int).tolist())

    precisions, recalls, ndcgs = [], [], []
    for user_id, real_items in truth.items():
        recs = collab.recommend(user_id, n=k)
        rec_items = [int(i) for i, _ in recs]
        if not rec_items:
            continue
        hits = sum(1 for it in rec_items if it in real_items)
        precisions.append(hits / k)
        recalls.append(hits / max(1, len(real_items)))
        ndcgs.append(_ndcg(rec_items, real_items, k))
    return {
        "precision_at_k": float(np.mean(precisions)) if precisions else 0.0,
        "recall_at_k": float(np.mean(recalls)) if recalls else 0.0,
        "ndcg_at_k": float(np.mean(ndcgs)) if ndcgs else 0.0,
        "k": k,
        "evaluated_users": len(precisions),
    }


def _ndcg(rec: Iterable[int], truth: set[int], k: int) -> float:
    dcg = 0.0
    for i, item in enumerate(rec):
        rel = 1.0 if item in truth else 0.0
        dcg += rel / np.log2(i + 2)
    ideal = sum(1.0 / np.log2(i + 2) for i in range(min(len(truth), k)))
    if ideal == 0:
        return 0.0
    return float(dcg / ideal)


def _metrics_to_json(d: dict) -> str:
    import json
    return json.dumps(d, indent=2, ensure_ascii=False)


def main() -> None:
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s: %(message)s")
    cfg = MLConfig()
    asyncio.run(run(cfg))


if __name__ == "__main__":
    main()
