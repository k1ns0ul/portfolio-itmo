from __future__ import annotations

import json
import logging
from datetime import datetime, timezone
from typing import Iterable

import numpy as np
import pandas as pd
from confluent_kafka import Producer

from config import MLConfig
from db import ClickHouseDB
from features import FEATURE_COLUMNS, FeatureExtractor
from models import EnsembleModel
from training.pipeline import load_serving

log = logging.getLogger(__name__)


class ScoringService:
    def __init__(self, cfg: MLConfig, db: ClickHouseDB | None = None) -> None:
        self.cfg = cfg
        self.db = db or ClickHouseDB(cfg)
        self.extractor = FeatureExtractor()
        self.ensemble, self.scaler, self.feature_columns = load_serving(cfg)
        log.info("scoring service ready (alpha=%.2f)", self.ensemble.alpha)
        self.producer = Producer({
            "bootstrap.servers": ",".join(cfg.kafka_brokers),
            "linger.ms": 50,
            "compression.type": "snappy",
        })

    def close(self) -> None:
        try:
            self.producer.flush(timeout=5)
        finally:
            self.db.close()

    def score_wallets(self, addresses: Iterable[str]) -> pd.DataFrame:
        addresses = [a for a in addresses if a]
        if not addresses:
            return pd.DataFrame()
        raw = self.db.fetch_wallet_features(addresses)
        if raw.empty:
            return pd.DataFrame()
        features = self.extractor.extract(raw)
        if features.empty:
            return pd.DataFrame()

        prepared = self._prepare(features)
        predictions = self.ensemble.predict_frame(prepared)
        merged = features.join(predictions, how="left")
        self._persist(merged)
        self._publish(merged)
        return merged

    def score_all(self) -> int:
        total = 0
        batch_size = self.cfg.batch_size
        wallets = self.db.list_all_wallets()
        log.info("scoring %d wallets in batches of %d", len(wallets), batch_size)
        for i in range(0, len(wallets), batch_size):
            chunk = wallets[i:i + batch_size]
            df = self.score_wallets(chunk)
            total += len(df)
            log.info("batch %d scored=%d total=%d", i // batch_size, len(df), total)
        return total

    def _prepare(self, features: pd.DataFrame) -> pd.DataFrame:
        df = features.copy()
        df = df.reindex(columns=self.feature_columns, fill_value=0.0)
        df = df.replace([np.inf, -np.inf], np.nan).fillna(0.0)
        scaled = self.scaler.transform(df.to_numpy(dtype=np.float32))
        return pd.DataFrame(scaled, columns=self.feature_columns, index=df.index)

    def _persist(self, merged: pd.DataFrame) -> None:
        now = datetime.now(timezone.utc)
        merged = merged.copy()
        merged["first_seen"] = now
        merged["last_seen"] = now
        merged["tx_count"] = merged.get("tx_count", 0)
        merged["unique_counterparties"] = merged.get("unique_counterparties", 0)
        merged["avg_tx_amount"] = merged.get("avg_tx_amount", 0.0)
        merged["median_tx_amount"] = merged.get("median_tx_amount", 0.0)
        merged["herfindahl_index"] = merged.get("herfindahl_index", 0.0)
        merged["smart_contract_ratio"] = merged.get("smart_contract_ratio", 0.0)
        merged["velocity_24h"] = merged.get("velocity_24h", 0.0)
        merged["dormancy_days"] = merged.get("dormancy_days", 0.0)
        self.db.write_scores(merged)
        history = merged[["score", "category"]].copy()
        self.db.write_score_history(history)

    def _publish(self, merged: pd.DataFrame) -> None:
        now = datetime.now(timezone.utc).isoformat()
        for wallet, row in merged.iterrows():
            payload = {
                "wallet": str(wallet),
                "score": float(row["score"]),
                "previous_score": 0.0,
                "category": str(row["category"]),
                "reason": "",
                "updated_at": now,
            }
            envelope = {
                "type": "score_updated",
                "ts": now,
                "source": "ml-scorer",
                "payload": payload,
            }
            self.producer.produce(
                self.cfg.kafka_topic_scores,
                key=str(wallet).encode("utf-8"),
                value=json.dumps(envelope).encode("utf-8"),
            )
        self.producer.poll(0)
