from __future__ import annotations

from abc import ABC, abstractmethod


class LLMBackend(ABC):
    name: str = "base"

    @abstractmethod
    def generate(self, prompt: str, max_tokens: int, temperature: float, top_p: float = 0.9) -> str:
        ...

    def ready(self) -> bool:
        return True

    def close(self) -> None:
        return None
