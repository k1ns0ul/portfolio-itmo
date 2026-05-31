from __future__ import annotations

import json
import logging
import signal
import time
import uuid
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

import clickhouse_connect
import numpy as np
from confluent_kafka import Consumer, KafkaError, Producer

from config import MLConfig
from models import AutoencoderModel, IForestModel
from training.pipeline import FEATURE_COLUMNS

log = logging.getLogger(__name__)


@dataclass
class _Feature:
    tx_id: str
    client_id: str
    vector: list[float]
    received_at: datetime


class InferenceWorker:
    def __init__(self, cfg: MLConfig) -> None:
        self.cfg = cfg
        self.iforest = IForestModel()
        self.autoencoder = AutoencoderModel()
        self.consumer: Consumer | None = None
        self.producer: Producer | None = None
        self.ch_client = None
        self.stop_event = False
        self._install_signals()

    def setup(self) -> None:
        model_dir = Path(self.cfg.model_dir)
        if not (model_dir / "iforest.joblib").exists():
            raise RuntimeError(f"models not found in {model_dir}; train first")
        self.iforest.load(model_dir)
        self.autoencoder.load(model_dir)

        self.consumer = Consumer({
            "bootstrap.servers": ",".join(self.cfg.kafka_brokers),
            "group.id": self.cfg.kafka_group,
            "auto.offset.reset": "latest",
            "enable.auto.commit": True,
        })
        self.consumer.subscribe([self.cfg.topic_features])

        self.producer = Producer({
            "bootstrap.servers": ",".join(self.cfg.kafka_brokers),
            "linger.ms": 50,
            "compression.type": "snappy",
        })

        self.ch_client = clickhouse_connect.get_client(
            host=self.cfg.clickhouse_host,
            port=self.cfg.clickhouse_port,
            username=self.cfg.clickhouse_user,
            password=self.cfg.clickhouse_password,
            database=self.cfg.clickhouse_db,
        )

    def run(self) -> None:
        self.setup()
        log.info("worker started: topic=%s group=%s threshold=%.5f",
                 self.cfg.topic_features, self.cfg.kafka_group, self.autoencoder.threshold)

        buffer: list[_Feature] = []
        last_flush = time.time()
        try:
            while not self.stop_event:
                msg = self.consumer.poll(0.5)
                now = time.time()
                if msg is None:
                    pass
                elif msg.error():
                    if msg.error().code() != KafkaError._PARTITION_EOF:
                        log.warning("kafka error: %s", msg.error())
                else:
                    feat = self._parse(msg.value())
                    if feat is not None:
                        buffer.append(feat)

                if buffer and (
                    len(buffer) >= self.cfg.batch_size
                    or now - last_flush >= self.cfg.batch_timeout_sec
                ):
                    self._process_batch(buffer)
                    buffer = []
                    last_flush = now
            if buffer:
                self._process_batch(buffer)
        finally:
            self.shutdown()

    def shutdown(self) -> None:
        if self.consumer:
            self.consumer.close()
        if self.producer:
            self.producer.flush(timeout=5)
        if self.ch_client:
            self.ch_client.close()
        log.info("worker stopped")

    def _process_batch(self, batch: list[_Feature]) -> None:
        if not batch:
            return
        X = np.array([f.vector for f in batch], dtype=np.float32)
        flags = self.iforest.predict(X)
        iforest_scores = self.iforest.scores(X)
        ae_errors = self.autoencoder.predict(X)

        alerts: list[dict[str, Any]] = []
        for i, feat in enumerate(batch):
            if flags[i] != -1:
                continue
            if ae_errors[i] <= self.autoencoder.threshold:
                continue
            score = float(0.5 * iforest_scores[i] + 0.5 * (ae_errors[i] / max(self.autoencoder.threshold, 1e-9)))
            level = self._level(score)
            alerts.append({
                "id": str(uuid.uuid4()),
                "tx_id": feat.tx_id,
                "client_id": feat.client_id,
                "score": score,
                "iforest_flag": True,
                "autoencoder_score": float(ae_errors[i]),
                "level": level,
                "created_at": datetime.now(timezone.utc),
            })

        log.info("batch processed size=%d alerts=%d", len(batch), len(alerts))
        if not alerts:
            return
        self._emit(alerts)
        self._persist(alerts)

    def _emit(self, alerts: list[dict[str, Any]]) -> None:
        for a in alerts:
            payload = {
                "id": a["id"],
                "tx_id": a["tx_id"],
                "client_id": a["client_id"],
                "score": a["score"],
                "iforest_flag": a["iforest_flag"],
                "autoencoder_score": a["autoencoder_score"],
                "level": a["level"],
                "created_at": a["created_at"].isoformat(),
            }
            self.producer.produce(
                self.cfg.topic_alerts,
                key=a["client_id"].encode("utf-8"),
                value=json.dumps(payload).encode("utf-8"),
            )
        self.producer.poll(0)

    def _persist(self, alerts: list[dict[str, Any]]) -> None:
        rows = [[
            a["id"], a["tx_id"], a["client_id"], a["score"],
            1 if a["iforest_flag"] else 0,
            a["autoencoder_score"], a["level"], a["created_at"],
        ] for a in alerts]
        cols = ["id", "tx_id", "client_id", "score", "iforest_flag",
                "autoencoder_score", "level", "created_at"]
        try:
            self.ch_client.insert("anomalies.alerts", rows, column_names=cols)
        except Exception as e:
            log.exception("clickhouse insert failed: %s", e)

    def _parse(self, raw: bytes) -> _Feature | None:
        try:
            obj = json.loads(raw.decode("utf-8"))
        except (UnicodeDecodeError, json.JSONDecodeError) as e:
            log.warning("decode error: %s", e)
            return None
        try:
            vector = [float(obj.get(c, 0.0)) for c in FEATURE_COLUMNS]
        except (TypeError, ValueError) as e:
            log.warning("vector build error: %s", e)
            return None
        return _Feature(
            tx_id=str(obj.get("tx_id", "")),
            client_id=str(obj.get("client_id", "")),
            vector=vector,
            received_at=datetime.now(timezone.utc),
        )

    def _level(self, score: float) -> str:
        if score >= 3.0:
            return "critical"
        if score >= 1.5:
            return "warning"
        return "info"

    def _install_signals(self) -> None:
        def stop(_signum, _frame):
            log.info("signal received, shutting down")
            self.stop_event = True
        signal.signal(signal.SIGINT, stop)
        signal.signal(signal.SIGTERM, stop)


def main() -> None:
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s: %(message)s")
    cfg = MLConfig()
    InferenceWorker(cfg).run()


if __name__ == "__main__":
    main()
