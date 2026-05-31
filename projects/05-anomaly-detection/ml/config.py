from __future__ import annotations

import os
from dataclasses import dataclass, field


def _env(key: str, default: str) -> str:
    return os.environ.get(key, default)


def _env_int(key: str, default: int) -> int:
    try:
        return int(os.environ[key])
    except (KeyError, ValueError):
        return default


def _env_float(key: str, default: float) -> float:
    try:
        return float(os.environ[key])
    except (KeyError, ValueError):
        return default


def _env_list(key: str, default: list[str]) -> list[str]:
    raw = os.environ.get(key)
    if not raw:
        return default
    return [p.strip() for p in raw.split(",") if p.strip()]


@dataclass
class MLConfig:
    kafka_brokers: list[str] = field(default_factory=lambda: _env_list("KAFKA_BROKERS", ["kafka:9092"]))
    topic_features: str = field(default_factory=lambda: _env("KAFKA_TOPIC_FEATURES", "tx-features"))
    topic_alerts: str = field(default_factory=lambda: _env("KAFKA_TOPIC_ALERTS", "anomaly-alerts"))
    kafka_group: str = field(default_factory=lambda: _env("KAFKA_GROUP_ML", "ml-worker"))

    clickhouse_host: str = field(default_factory=lambda: _env("CLICKHOUSE_HOST", "clickhouse"))
    clickhouse_port: int = field(default_factory=lambda: _env_int("CLICKHOUSE_PORT", 8123))
    clickhouse_user: str = field(default_factory=lambda: _env("CLICKHOUSE_USER", "default"))
    clickhouse_password: str = field(default_factory=lambda: _env("CLICKHOUSE_PASSWORD", ""))
    clickhouse_db: str = field(default_factory=lambda: _env("CLICKHOUSE_DB", "anomalies"))

    model_dir: str = field(default_factory=lambda: _env("ML_MODEL_DIR", "./artifacts"))
    batch_size: int = field(default_factory=lambda: _env_int("ML_BATCH_SIZE", 100))
    batch_timeout_sec: float = field(default_factory=lambda: _env_float("ML_BATCH_TIMEOUT", 2.0))

    iforest_contamination: float = field(default_factory=lambda: _env_float("IF_CONTAM", 0.01))
    iforest_estimators: int = field(default_factory=lambda: _env_int("IF_ESTIMATORS", 200))

    autoencoder_epochs: int = field(default_factory=lambda: _env_int("AE_EPOCHS", 100))
    autoencoder_batch: int = field(default_factory=lambda: _env_int("AE_BATCH", 512))
    autoencoder_lr: float = field(default_factory=lambda: _env_float("AE_LR", 0.001))
    autoencoder_patience: int = field(default_factory=lambda: _env_int("AE_PATIENCE", 15))

    score_threshold_percentile: float = field(default_factory=lambda: _env_float("SCORE_PERCENTILE", 99.0))
    log_level: str = field(default_factory=lambda: _env("LOG_LEVEL", "INFO"))
    random_state: int = field(default_factory=lambda: _env_int("RANDOM_STATE", 42))
