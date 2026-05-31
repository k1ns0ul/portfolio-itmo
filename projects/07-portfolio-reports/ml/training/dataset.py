from __future__ import annotations

import json
import logging
import random
from dataclasses import dataclass
from pathlib import Path

from prompts import SYSTEM_PROMPT, build_prompt, compact_portfolio

log = logging.getLogger(__name__)


@dataclass
class ExampleSpec:
    profile: str
    portfolio: dict
    reference_report: str


_PROFILES = ["whale", "active_trader", "hodler", "degen", "new_wallet"]


class DatasetBuilder:
    def __init__(self, seed: int = 42) -> None:
        self.rng = random.Random(seed)

    def generate_examples(self, n: int = 500) -> list[dict]:
        examples: list[dict] = []
        for i in range(n):
            profile = self.rng.choice(_PROFILES)
            portfolio = self._mock_portfolio(profile, i)
            reference = self._reference_text(profile, portfolio)
            examples.append({
                "input": json.dumps(compact_portfolio(portfolio), ensure_ascii=False),
                "output": reference,
                "profile": profile,
                "portfolio": portfolio,
            })
        return examples

    def save_jsonl(self, examples: list[dict], path: str | Path) -> None:
        path = Path(path)
        path.parent.mkdir(parents=True, exist_ok=True)
        with path.open("w", encoding="utf-8") as f:
            for ex in examples:
                portfolio = ex["portfolio"]
                user = (
                    "Сгенерируй структурированный отчет по портфелю кошелька. "
                    f"Исходные метрики:\n```json\n"
                    f"{json.dumps(compact_portfolio(portfolio), ensure_ascii=False, indent=2)}\n```"
                )
                line = {
                    "messages": [
                        {"role": "system", "content": SYSTEM_PROMPT},
                        {"role": "user", "content": user},
                        {"role": "assistant", "content": ex["output"]},
                    ],
                    "profile": ex["profile"],
                }
                f.write(json.dumps(line, ensure_ascii=False) + "\n")
        log.info("dataset saved to %s (%d examples)", path, len(examples))

    def build_prompt_pairs(self, examples: list[dict]) -> list[tuple[str, str]]:
        return [(build_prompt(ex["portfolio"]), ex["output"]) for ex in examples]

    def _mock_portfolio(self, profile: str, seed_offset: int) -> dict:
        rng = random.Random(self.rng.random() + seed_offset)
        if profile == "whale":
            total = rng.uniform(500_000, 5_000_000)
            token_count = rng.randint(10, 14)
            stable_pct = rng.uniform(20, 40)
            risk_score = rng.uniform(15, 35)
        elif profile == "active_trader":
            total = rng.uniform(30_000, 250_000)
            token_count = rng.randint(6, 10)
            stable_pct = rng.uniform(5, 25)
            risk_score = rng.uniform(35, 65)
        elif profile == "hodler":
            total = rng.uniform(2_000, 30_000)
            token_count = rng.randint(2, 5)
            stable_pct = rng.uniform(10, 50)
            risk_score = rng.uniform(15, 40)
        elif profile == "degen":
            total = rng.uniform(500, 20_000)
            token_count = rng.randint(8, 14)
            stable_pct = rng.uniform(0, 10)
            risk_score = rng.uniform(70, 95)
        else:
            total = rng.uniform(50, 1_000)
            token_count = rng.randint(1, 2)
            stable_pct = rng.uniform(50, 100)
            risk_score = rng.uniform(5, 25)

        pnl_total = total * rng.uniform(-0.4, 0.6)
        pnl_realized = pnl_total * rng.uniform(0.2, 0.5)
        pnl_unrealized = pnl_total - pnl_realized

        top_pct = rng.uniform(15, 80) if profile != "whale" else rng.uniform(10, 25)
        herf = top_pct * top_pct / 10000.0 + (token_count - 1) * 0.005

        factors: list[str] = []
        if top_pct > 60:
            factors.append("высокая концентрация в одном токене")
        if stable_pct < 5:
            factors.append("нет защитной позиции в стейблкоинах")
        if profile == "degen":
            factors.append("есть подозрительные или скам токены")

        risk_level = "low"
        if risk_score >= 70:
            risk_level = "critical"
        elif risk_score >= 50:
            risk_level = "high"
        elif risk_score >= 30:
            risk_level = "medium"

        return {
            "address": f"mock-{profile}-{seed_offset:04d}",
            "total_value": round(total, 2),
            "tokens": [],
            "pnl": {
                "realized": round(pnl_realized, 2),
                "unrealized": round(pnl_unrealized, 2),
                "total": round(pnl_total, 2),
                "pct_return": round(pnl_total / max(1.0, total) * 100, 2),
            },
            "diversification": {
                "herfindahl_index": round(herf, 4),
                "token_count": token_count,
                "stablecoin_pct": round(stable_pct, 2),
                "blue_chip_pct": round(rng.uniform(0, 30), 2),
                "top_token_pct": round(top_pct, 2),
                "concentration_level": "high" if herf >= 0.4 else ("medium" if herf >= 0.15 else "low"),
            },
            "risk": {
                "score": round(risk_score, 2),
                "level": risk_level,
                "volatile_pct": round(100 - stable_pct, 2),
                "scam_token_pct": round(rng.uniform(0, 25) if profile == "degen" else rng.uniform(0, 5), 2),
                "factors": factors,
            },
        }

    def _reference_text(self, profile: str, portfolio: dict) -> str:
        div = portfolio["diversification"]
        pnl = portfolio["pnl"]
        risk = portfolio["risk"]
        total = portfolio["total_value"]

        intros = {
            "whale": "Крупный портфель с продуманной структурой.",
            "active_trader": "Активный трейдер с разнообразными позициями.",
            "hodler": "Консервативная стратегия удержания базовых активов.",
            "degen": "Высокорисковый портфель с большой долей мелких токенов.",
            "new_wallet": "Молодой кошелек с минимальной историей.",
        }
        intro = intros.get(profile, "Портфель кошелька.")

        text = (
            "Краткое резюме:\n"
            f"{intro} Общая стоимость {total:.2f} USD, токенов в составе {div['token_count']}. "
            f"Уровень риска: {risk['level']}.\n\n"
            "Состав портфеля:\n"
            f"Доля стейблкоинов {div['stablecoin_pct']:.1f} процентов, голубых фишек {div['blue_chip_pct']:.1f} процентов. "
            f"Топ-позиция занимает {div['top_token_pct']:.1f} процентов от капитала.\n\n"
            "Доходность:\n"
            f"Реализованный результат {pnl['realized']:.2f} USD, нереализованный {pnl['unrealized']:.2f} USD. "
            f"Итоговый PnL {pnl['total']:.2f} USD, что составляет {pnl['pct_return']:.1f} процентов от вложении.\n\n"
            "Диверсификация:\n"
            f"Индекс Херфиндаля {div['herfindahl_index']:.3f}, уровень концентрации {div['concentration_level']}.\n\n"
            "Риски:\n"
            f"Скор риска {risk['score']:.1f} из 100. Доля волатильных активов {risk['volatile_pct']:.1f} процентов, "
            f"подозрительных токенов {risk['scam_token_pct']:.1f} процентов."
        )
        if risk["factors"]:
            text += "\nКлючевые факторы: " + "; ".join(risk["factors"]) + "."
        text += "\n\nРекомендации:\n" + self._recommendation(profile, risk["level"])
        return text

    def _recommendation(self, profile: str, level: str) -> str:
        if level == "critical":
            return "Сократить долю подозрительных активов и закрепить часть капитала в стейблкоинах."
        if level == "high":
            return "Увеличить долю защитных активов до 20-30 процентов и пересмотреть концентрацию."
        if profile == "hodler":
            return "Сохранять текущую стратегию удержания и периодически пополнять позиции."
        if profile == "degen":
            return "Ограничить вход в новые мелкие токены и зафиксировать прибыль по выросшим позициям."
        return "Удерживать текущую структуру, отслеживать концентрацию и регулярно проверять PnL."
