from __future__ import annotations

from config import LLMConfig
from inference import MockBackend
from prompts import build_prompt


def _portfolio() -> dict:
    return {
        "address": "ADDR-TEST",
        "total_value": 9999.99,
        "tokens": [],
        "pnl": {"realized": 100.0, "unrealized": 50.0, "total": 150.0, "pct_return": 1.5},
        "diversification": {
            "herfindahl_index": 0.18, "token_count": 7,
            "stablecoin_pct": 22.5, "blue_chip_pct": 30.0,
            "top_token_pct": 35.0, "concentration_level": "medium",
        },
        "risk": {"score": 42.0, "level": "high", "volatile_pct": 77.5, "scam_token_pct": 0.0, "factors": []},
    }


def test_mock_uses_input_numbers():
    cfg = LLMConfig(backend="mock")
    backend = MockBackend(cfg)
    prompt = build_prompt(_portfolio())
    text = backend.generate(prompt, max_tokens=400, temperature=0.0)
    assert "9999.99" in text
    assert "ADDR-TEST" in text
    assert "high" in text
    assert "Краткое резюме" in text


def test_mock_respects_max_tokens_loosely():
    cfg = LLMConfig(backend="mock")
    backend = MockBackend(cfg)
    prompt = build_prompt(_portfolio())
    text = backend.generate(prompt, max_tokens=10, temperature=0.0)
    assert len(text) <= 10 * 4 + 1


def test_mock_returns_text_without_portfolio_data():
    cfg = LLMConfig(backend="mock")
    backend = MockBackend(cfg)
    text = backend.generate("<|user|>hi<|assistant|>\n", max_tokens=200, temperature=0.0)
    assert "Краткое резюме" in text
