from __future__ import annotations

from prompts import REPORT_SECTIONS, SYSTEM_PROMPT, build_prompt, compact_portfolio, extract_summary


def _sample_portfolio() -> dict:
    return {
        "address": "ADDR123",
        "total_value": 12345.67,
        "tokens": [
            {"mint": "M1", "symbol": "USDC", "value_usd": 5000.0,
             "pct_of_portfolio": 40.5, "pnl": 100.0, "is_stablecoin": True, "risk_category": "legit"},
            {"mint": "M2", "symbol": "SOL", "value_usd": 4000.0,
             "pct_of_portfolio": 32.4, "pnl": -50.0, "is_stablecoin": False, "risk_category": "legit"},
        ],
        "pnl": {"realized": 100.0, "unrealized": -50.0, "total": 50.0, "pct_return": 0.4},
        "diversification": {
            "herfindahl_index": 0.25, "token_count": 2,
            "stablecoin_pct": 40.5, "blue_chip_pct": 32.4,
            "top_token_pct": 40.5, "concentration_level": "medium",
        },
        "risk": {
            "score": 35.0, "level": "medium",
            "volatile_pct": 59.5, "scam_token_pct": 0.0,
            "factors": ["низкая диверсификация"],
        },
    }


def test_compact_portfolio_keeps_numbers():
    c = compact_portfolio(_sample_portfolio())
    assert c["total_value"] == 12345.67
    assert c["token_count"] == 2
    assert c["risk_score"] == 35.0
    assert len(c["top_positions"]) == 2


def test_build_prompt_contains_metrics():
    prompt = build_prompt(_sample_portfolio())
    assert "12345.67" in prompt
    assert "ADDR123" in prompt
    assert "Краткое резюме" not in prompt or prompt.startswith("<|system|>")
    assert "<|user|>" in prompt and "<|assistant|>" in prompt


def test_system_prompt_lists_all_sections():
    for section in REPORT_SECTIONS:
        assert section in SYSTEM_PROMPT


def test_extract_summary_finds_resume_block():
    text = "Краткое резюме:\nЭто короткий текст.\n\nСостав портфеля:\nДетали."
    summary = extract_summary(text)
    assert "короткий текст" in summary
