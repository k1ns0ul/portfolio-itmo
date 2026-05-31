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
    pg_dsn: str = field(default_factory=lambda: _env(
        "PG_DSN",
        "postgresql://app:app@postgres:5432/tvygoda",
    ))
    model_dir: str = field(default_factory=lambda: _env("ML_MODEL_DIR", "./artifacts"))

    server_host: str = field(default_factory=lambda: _env("ML_HOST", "0.0.0.0"))
    server_port: int = field(default_factory=lambda: _env_int("ML_PORT", 8090))

    als_factors: int = field(default_factory=lambda: _env_int("ALS_FACTORS", 32))
    als_iters: int = field(default_factory=lambda: _env_int("ALS_ITERS", 15))
    als_reg: float = field(default_factory=lambda: _env_float("ALS_REG", 0.1))
    als_alpha: float = field(default_factory=lambda: _env_float("ALS_ALPHA", 40.0))

    refresh_batch_size: int = field(default_factory=lambda: _env_int("REFRESH_BATCH", 1000))
    refresh_top_n: int = field(default_factory=lambda: _env_int("REFRESH_TOP_N", 20))

    fraud_contamination: float = field(default_factory=lambda: _env_float("FRAUD_CONT", 0.05))
    fraud_velocity_threshold: int = field(default_factory=lambda: _env_int("FRAUD_VELOCITY", 10))
    fraud_window_hours: int = field(default_factory=lambda: _env_int("FRAUD_WINDOW", 24))
    fraud_ring_max_length: int = field(default_factory=lambda: _env_int("FRAUD_RING_MAX", 5))

    log_level: str = field(default_factory=lambda: _env("LOG_LEVEL", "INFO"))
    random_state: int = field(default_factory=lambda: _env_int("RANDOM_STATE", 42))
