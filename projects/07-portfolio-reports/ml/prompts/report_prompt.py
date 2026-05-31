from __future__ import annotations

import json
from typing import Any

SYSTEM_PROMPT = (
    "Ты финансовый аналитик. Пиши на русском языке. "
    "Структура отчета строго фиксирована: 1) Краткое резюме в 2-3 предложениях, "
    "2) Состав портфеля, 3) Доходность, 4) Диверсификация, 5) Риски, 6) Рекомендации. "
    "Используй только цифры из входных данных, не выдумывай новые значения. "
    "Стиль строгий, без эмоциональных оценок. "
)

REPORT_SECTIONS = [
    "Краткое резюме",
    "Состав портфеля",
    "Доходность",
    "Диверсификация",
    "Риски",
    "Рекомендации",
]


def compact_portfolio(portfolio: dict[str, Any]) -> dict[str, Any]:
    tokens = portfolio.get("tokens") or []
    top = []
    for t in tokens[:5]:
        top.append({
            "symbol": t.get("symbol", ""),
            "mint": t.get("mint", ""),
            "value_usd": round(float(t.get("value_usd", 0.0)), 2),
            "pct_of_portfolio": round(float(t.get("pct_of_portfolio", 0.0)), 2),
            "pnl": round(float(t.get("pnl", 0.0)), 2),
            "is_stablecoin": bool(t.get("is_stablecoin", False)),
            "risk_category": str(t.get("risk_category", "legit")),
        })
    pnl = portfolio.get("pnl", {}) or {}
    div = portfolio.get("diversification", {}) or {}
    risk = portfolio.get("risk", {}) or {}
    return {
        "address": portfolio.get("address", ""),
        "total_value": round(float(portfolio.get("total_value", 0.0)), 2),
        "token_count": int(div.get("token_count", len(tokens))),
        "stablecoin_pct": round(float(div.get("stablecoin_pct", 0.0)), 2),
        "blue_chip_pct": round(float(div.get("blue_chip_pct", 0.0)), 2),
        "top_token_pct": round(float(div.get("top_token_pct", 0.0)), 2),
        "herfindahl_index": round(float(div.get("herfindahl_index", 0.0)), 4),
        "concentration_level": str(div.get("concentration_level", "")),
        "pnl_realized": round(float(pnl.get("realized", 0.0)), 2),
        "pnl_unrealized": round(float(pnl.get("unrealized", 0.0)), 2),
        "pnl_total": round(float(pnl.get("total", 0.0)), 2),
        "pnl_pct_return": round(float(pnl.get("pct_return", 0.0)), 2),
        "risk_score": round(float(risk.get("score", 0.0)), 2),
        "risk_level": str(risk.get("level", "")),
        "volatile_pct": round(float(risk.get("volatile_pct", 0.0)), 2),
        "scam_token_pct": round(float(risk.get("scam_token_pct", 0.0)), 2),
        "risk_factors": list(risk.get("factors") or []),
        "top_positions": top,
    }


def build_prompt(portfolio: dict[str, Any]) -> str:
    compact = compact_portfolio(portfolio)
    payload = json.dumps(compact, ensure_ascii=False, indent=2)
    return (
        f"<|system|>\n{SYSTEM_PROMPT}\n"
        "<|user|>\n"
        "Сгенерируй структурированный отчет по портфелю кошелька. Исходные метрики:\n"
        f"```json\n{payload}\n```\n"
        "Используй все шесть секций в указанном порядке. Каждую секцию начинай с ее заголовка.\n"
        "<|assistant|>\n"
    )


def extract_summary(text: str) -> str:
    if not text:
        return ""
    marker = "Краткое резюме"
    idx = text.find(marker)
    if idx < 0:
        first_line = text.strip().split("\n", 1)[0]
        return first_line[:240]
    chunk = text[idx + len(marker):]
    end = chunk.find("\n\n")
    if end < 0:
        end = min(len(chunk), 400)
    summary = chunk[:end].strip(": \n")
    return summary[:300]
