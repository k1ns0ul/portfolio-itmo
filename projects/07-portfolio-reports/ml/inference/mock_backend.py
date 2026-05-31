from __future__ import annotations

import logging
import re

from config import LLMConfig
from inference.base import LLMBackend

log = logging.getLogger(__name__)


class MockBackend(LLMBackend):
    name = "mock"

    def __init__(self, cfg: LLMConfig) -> None:
        self.cfg = cfg
        log.info("mock backend initialized")

    def generate(self, prompt: str, max_tokens: int, temperature: float, top_p: float = 0.9) -> str:
        portfolio = self._extract_portfolio(prompt)
        total_value = portfolio.get("total_value", 0.0)
        token_count = portfolio.get("token_count", 0)
        pnl_total = portfolio.get("pnl_total", 0.0)
        herf = portfolio.get("herfindahl_index", 0.0)
        risk_level = portfolio.get("risk_level", "low")
        risk_score = portfolio.get("risk_score", 0.0)
        stable_pct = portfolio.get("stablecoin_pct", 0.0)
        top_pct = portfolio.get("top_token_pct", 0.0)
        address = portfolio.get("address", "unknown")

        text = (
            "Краткое резюме:\n"
            f"Кошелек {address} держит позиции на сумму {total_value:.2f} USD, "
            f"всего {int(token_count)} токенов. Уровень риска: {risk_level}.\n\n"
            "Состав портфеля:\n"
            f"Доля стейблкоинов {stable_pct:.1f} процентов. Доля топ-токена {top_pct:.1f} процентов.\n\n"
            "Доходность:\n"
            f"Совокупный PnL: {pnl_total:.2f} USD.\n\n"
            "Диверсификация:\n"
            f"Индекс Херфиндаля {herf:.3f}.\n\n"
            "Риски:\n"
            f"Скор риска {risk_score:.1f} из 100.\n\n"
            "Рекомендации:\n"
            "Сохранять текущее распределение и регулярно проверять долю волатильных позиций."
        )
        return text[: max_tokens * 4]

    def _extract_portfolio(self, prompt: str) -> dict:
        m = re.search(r"```json\n(.+?)\n```", prompt, re.DOTALL)
        if not m:
            return {}
        import json
        try:
            return json.loads(m.group(1))
        except json.JSONDecodeError:
            return {}
