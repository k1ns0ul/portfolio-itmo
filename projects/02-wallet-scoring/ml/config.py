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
    clickhouse_host: str = field(default_factory=lambda: _env("CLICKHOUSE_HOST", "clickhouse"))
    clickhouse_port: int = field(default_factory=lambda: _env_int("CLICKHOUSE_PORT", 8123))
    clickhouse_user: str = field(default_factory=lambda: _env("CLICKHOUSE_USER", "default"))
    clickhouse_password: str = field(default_factory=lambda: _env("CLICKHOUSE_PASSWORD", ""))
    clickhouse_db: str = field(default_factory=lambda: _env("CLICKHOUSE_DB", "wallets"))

    kafka_brokers: list[str] = field(default_factory=lambda: _env_list("KAFKA_BROKERS", ["kafka:9092"]))
    kafka_topic_scores: str = field(default_factory=lambda: _env("KAFKA_TOPIC_SCORES", "score-updates"))
    kafka_topic_requests: str = field(default_factory=lambda: _env("KAFKA_TOPIC_REQUESTS", "scoring-requests"))
    kafka_group: str = field(default_factory=lambda: _env("KAFKA_ML_GROUP", "ml-scorer"))

    model_dir: str = field(default_factory=lambda: _env("ML_MODEL_DIR", "./artifacts"))

    batch_size: int = field(default_factory=lambda: _env_int("ML_BATCH_SIZE", 10000))
    daemon_interval_sec: int = field(default_factory=lambda: _env_int("ML_DAEMON_INTERVAL", 300))

    xgb_estimators: int = field(default_factory=lambda: _env_int("XGB_ESTIMATORS", 300))
    xgb_lr: float = field(default_factory=lambda: _env_float("XGB_LR", 0.1))
    xgb_max_depth: int = field(default_factory=lambda: _env_int("XGB_MAX_DEPTH", 6))

    nn_epochs: int = field(default_factory=lambda: _env_int("NN_EPOCHS", 50))
    nn_batch_size: int = field(default_factory=lambda: _env_int("NN_BATCH", 256))
    nn_lr: float = field(default_factory=lambda: _env_float("NN_LR", 0.001))
    nn_patience: int = field(default_factory=lambda: _env_int("NN_PATIENCE", 10))

    random_state: int = field(default_factory=lambda: _env_int("RANDOM_STATE", 42))

    log_level: str = field(default_factory=lambda: _env("LOG_LEVEL", "INFO"))
