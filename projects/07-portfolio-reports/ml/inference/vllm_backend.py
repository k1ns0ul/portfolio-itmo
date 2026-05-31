from __future__ import annotations

import logging

import httpx

from config import LLMConfig
from inference.base import LLMBackend

log = logging.getLogger(__name__)


class VLLMBackend(LLMBackend):
    name = "vllm"

    def __init__(self, cfg: LLMConfig) -> None:
        self.cfg = cfg
        self.client = httpx.Client(base_url=cfg.vllm_url, timeout=cfg.request_timeout)
        self.model = cfg.vllm_model_name
        log.info("vllm backend at %s (model=%s)", cfg.vllm_url, cfg.vllm_model_name)

    def generate(self, prompt: str, max_tokens: int, temperature: float, top_p: float = 0.9) -> str:
        payload = {
            "model": self.model,
            "prompt": prompt,
            "max_tokens": max_tokens,
            "temperature": temperature,
            "top_p": top_p,
            "stream": False,
        }
        resp = self.client.post("/v1/completions", json=payload)
        if resp.status_code >= 500:
            raise RuntimeError(f"vllm 5xx: {resp.text}")
        resp.raise_for_status()
        body = resp.json()
        choices = body.get("choices") or []
        if not choices:
            raise RuntimeError("vllm returned no choices")
        return str(choices[0].get("text", "")).strip()

    def ready(self) -> bool:
        try:
            r = self.client.get("/v1/models")
            return r.status_code == 200
        except httpx.HTTPError:
            return False

    def close(self) -> None:
        self.client.close()
