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


@dataclass
class MLConfig:
    clickhouse_host: str = field(default_factory=lambda: _env("CLICKHOUSE_HOST", "clickhouse"))
    clickhouse_port: int = field(default_factory=lambda: _env_int("CLICKHOUSE_PORT", 8123))
    clickhouse_user: str = field(default_factory=lambda: _env("CLICKHOUSE_USER", "default"))
    clickhouse_password: str = field(default_factory=lambda: _env("CLICKHOUSE_PASSWORD", ""))
    clickhouse_db: str = field(default_factory=lambda: _env("CLICKHOUSE_DB", "orderflow"))

    model_dir: str = field(default_factory=lambda: _env("ML_MODEL_DIR", "./artifacts"))

    interval_sec: int = field(default_factory=lambda: _env_int("INTERVAL_SEC", 60))
    history_limit: int = field(default_factory=lambda: _env_int("HISTORY_LIMIT", 50000))
    target_horizon_sec: int = field(default_factory=lambda: _env_int("TARGET_HORIZON_SEC", 300))
    move_threshold: float = field(default_factory=lambda: _env_float("MOVE_THRESHOLD", 0.003))

    xgb_estimators: int = field(default_factory=lambda: _env_int("XGB_ESTIMATORS", 200))
    xgb_max_depth: int = field(default_factory=lambda: _env_int("XGB_MAX_DEPTH", 5))
    xgb_lr: float = field(default_factory=lambda: _env_float("XGB_LR", 0.1))

    lstm_seq_len: int = field(default_factory=lambda: _env_int("LSTM_SEQ", 12))
    lstm_hidden: int = field(default_factory=lambda: _env_int("LSTM_HIDDEN", 64))
    lstm_layers: int = field(default_factory=lambda: _env_int("LSTM_LAYERS", 2))
    lstm_dropout: float = field(default_factory=lambda: _env_float("LSTM_DROPOUT", 0.2))
    lstm_epochs: int = field(default_factory=lambda: _env_int("LSTM_EPOCHS", 80))
    lstm_batch: int = field(default_factory=lambda: _env_int("LSTM_BATCH", 64))
    lstm_lr: float = field(default_factory=lambda: _env_float("LSTM_LR", 0.001))
    lstm_patience: int = field(default_factory=lambda: _env_int("LSTM_PATIENCE", 10))

    daemon_interval_sec: int = field(default_factory=lambda: _env_int("DAEMON_INTERVAL", 60))
    min_confidence: float = field(default_factory=lambda: _env_float("MIN_CONFIDENCE", 0.4))

    random_state: int = field(default_factory=lambda: _env_int("RANDOM_STATE", 42))
    log_level: str = field(default_factory=lambda: _env("LOG_LEVEL", "INFO"))
