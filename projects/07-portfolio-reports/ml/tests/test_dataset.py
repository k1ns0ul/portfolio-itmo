from __future__ import annotations

import json

from training import DatasetBuilder


def test_generate_examples_count_and_fields():
    builder = DatasetBuilder(seed=1)
    examples = builder.generate_examples(n=10)
    assert len(examples) == 10
    for ex in examples:
        assert ex["profile"] in {"whale", "active_trader", "hodler", "degen", "new_wallet"}
        assert "input" in ex and "output" in ex and "portfolio" in ex
        assert "Краткое резюме" in ex["output"]
        assert "Состав портфеля" in ex["output"]
        assert "Рекомендации" in ex["output"]


def test_save_jsonl_produces_messages_format(tmp_path):
    builder = DatasetBuilder(seed=2)
    examples = builder.generate_examples(n=5)
    out = tmp_path / "dataset.jsonl"
    builder.save_jsonl(examples, out)

    lines = out.read_text(encoding="utf-8").strip().splitlines()
    assert len(lines) == 5
    for raw in lines:
        obj = json.loads(raw)
        messages = obj["messages"]
        roles = [m["role"] for m in messages]
        assert roles == ["system", "user", "assistant"]
        assert obj["profile"] in {"whale", "active_trader", "hodler", "degen", "new_wallet"}


def test_build_prompt_pairs_returns_str_pairs():
    builder = DatasetBuilder(seed=3)
    examples = builder.generate_examples(n=3)
    pairs = builder.build_prompt_pairs(examples)
    assert len(pairs) == 3
    for prompt, output in pairs:
        assert "<|user|>" in prompt
        assert isinstance(output, str) and output
