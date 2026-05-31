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
class LLMConfig:
    backend: str = field(default_factory=lambda: _env("LLM_BACKEND", "mock"))
    base_model: str = field(default_factory=lambda: _env("LLM_BASE_MODEL", "meta-llama/Meta-Llama-3-8B-Instruct"))
    adapter_path: str = field(default_factory=lambda: _env("LLM_ADAPTER", "/app/artifacts/adapter"))
    gguf_path: str = field(default_factory=lambda: _env("LLM_GGUF", "/app/artifacts/model.gguf"))
    vllm_url: str = field(default_factory=lambda: _env("VLLM_URL", "http://vllm:8000"))
    vllm_model_name: str = field(default_factory=lambda: _env("VLLM_MODEL", "llama3-portfolio"))

    server_host: str = field(default_factory=lambda: _env("LLM_HOST", "0.0.0.0"))
    server_port: int = field(default_factory=lambda: _env_int("LLM_PORT", 8090))

    max_tokens: int = field(default_factory=lambda: _env_int("LLM_MAX_TOKENS", 800))
    temperature: float = field(default_factory=lambda: _env_float("LLM_TEMPERATURE", 0.4))
    top_p: float = field(default_factory=lambda: _env_float("LLM_TOP_P", 0.9))
    request_timeout: float = field(default_factory=lambda: _env_float("LLM_REQUEST_TIMEOUT", 60.0))

    dataset_size: int = field(default_factory=lambda: _env_int("LLM_DATASET_SIZE", 500))
    epochs: int = field(default_factory=lambda: _env_int("LLM_EPOCHS", 3))
    batch_size: int = field(default_factory=lambda: _env_int("LLM_BATCH", 2))
    grad_accum: int = field(default_factory=lambda: _env_int("LLM_GRAD_ACCUM", 4))
    learning_rate: float = field(default_factory=lambda: _env_float("LLM_LR", 2e-4))
    warmup_ratio: float = field(default_factory=lambda: _env_float("LLM_WARMUP", 0.05))

    artifacts_dir: str = field(default_factory=lambda: _env("LLM_MODEL_DIR", "./artifacts"))
    log_level: str = field(default_factory=lambda: _env("LOG_LEVEL", "INFO"))
    random_state: int = field(default_factory=lambda: _env_int("RANDOM_STATE", 42))
