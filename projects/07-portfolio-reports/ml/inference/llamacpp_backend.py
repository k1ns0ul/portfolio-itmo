from __future__ import annotations

import logging
from pathlib import Path

from config import LLMConfig
from inference.base import LLMBackend

log = logging.getLogger(__name__)


class LlamaCppBackend(LLMBackend):
    name = "llamacpp"

    def __init__(self, cfg: LLMConfig) -> None:
        self.cfg = cfg
        path = Path(cfg.gguf_path)
        if not path.exists():
            raise FileNotFoundError(f"gguf model not found at {path}")
        try:
            from llama_cpp import Llama
        except ImportError as e:
            raise RuntimeError("llama-cpp-python is required for llamacpp backend") from e
        self.llama = Llama(
            model_path=str(path),
            n_ctx=4096,
            n_threads=None,
            verbose=False,
        )
        log.info("llamacpp backend loaded from %s", path)

    def generate(self, prompt: str, max_tokens: int, temperature: float, top_p: float = 0.9) -> str:
        out = self.llama(
            prompt,
            max_tokens=max_tokens,
            temperature=temperature,
            top_p=top_p,
            stop=["</report>"],
        )
        choices = out.get("choices") or []
        if not choices:
            raise RuntimeError("llamacpp returned no choices")
        return str(choices[0].get("text", "")).strip()

    def close(self) -> None:
        self.llama = None
