from __future__ import annotations

import json
import logging
import re
from dataclasses import asdict, dataclass, field
from pathlib import Path

from config import LLMConfig
from inference import LLMBackend
from prompts import REPORT_SECTIONS, build_prompt, compact_portfolio

log = logging.getLogger(__name__)


@dataclass
class EvalReport:
    examples: int = 0
    rouge_l_mean: float = 0.0
    factual_accuracy_mean: float = 0.0
    format_compliance: float = 0.0
    worst_cases: list[dict] = field(default_factory=list)


class EvalPipeline:
    def __init__(self, cfg: LLMConfig, backend: LLMBackend) -> None:
        self.cfg = cfg
        self.backend = backend

    def run(self, dataset_path: str | Path, limit: int = 50) -> EvalReport:
        path = Path(dataset_path)
        if not path.exists():
            raise FileNotFoundError(f"dataset not found at {path}")
        from rouge_score import rouge_scorer

        scorer = rouge_scorer.RougeScorer(["rougeL"], use_stemmer=False)
        rouge_scores: list[float] = []
        factual_scores: list[float] = []
        format_scores: list[float] = []
        worst: list[dict] = []

        with path.open("r", encoding="utf-8") as f:
            lines = list(f)
        examples = lines[:limit]

        for raw in examples:
            obj = json.loads(raw)
            portfolio = obj.get("portfolio") or self._portfolio_from_messages(obj)
            reference = obj["messages"][-1]["content"]
            prompt = build_prompt(portfolio)
            generated = self.backend.generate(
                prompt,
                max_tokens=self.cfg.max_tokens,
                temperature=0.1,
                top_p=self.cfg.top_p,
            )
            score = scorer.score(reference, generated)["rougeL"].fmeasure
            rouge_scores.append(score)
            fact = self._factual_accuracy(compact_portfolio(portfolio), generated)
            factual_scores.append(fact)
            fmt = self._format_compliance(generated)
            format_scores.append(fmt)
            combined = 0.5 * score + 0.5 * fact
            if len(worst) < 5:
                worst.append({
                    "address": portfolio.get("address", ""),
                    "rouge_l": round(score, 3),
                    "factual": round(fact, 3),
                    "format": round(fmt, 3),
                    "combined": round(combined, 3),
                    "generated": generated[:500],
                })
            else:
                worst.sort(key=lambda x: x["combined"])
                if combined < worst[-1]["combined"]:
                    worst[-1] = {
                        "address": portfolio.get("address", ""),
                        "rouge_l": round(score, 3),
                        "factual": round(fact, 3),
                        "format": round(fmt, 3),
                        "combined": round(combined, 3),
                        "generated": generated[:500],
                    }

        return EvalReport(
            examples=len(examples),
            rouge_l_mean=float(sum(rouge_scores) / max(1, len(rouge_scores))),
            factual_accuracy_mean=float(sum(factual_scores) / max(1, len(factual_scores))),
            format_compliance=float(sum(format_scores) / max(1, len(format_scores))),
            worst_cases=sorted(worst, key=lambda x: x["combined"]),
        )

    def _portfolio_from_messages(self, obj: dict) -> dict:
        for msg in obj.get("messages", []):
            if msg.get("role") == "user":
                content = msg.get("content", "")
                m = re.search(r"```json\n(.+?)\n```", content, re.DOTALL)
                if m:
                    return json.loads(m.group(1))
        return {}

    def _factual_accuracy(self, compact: dict, text: str) -> float:
        numeric_keys = [
            ("total_value", 5.0),
            ("token_count", 0.0),
            ("stablecoin_pct", 1.0),
            ("top_token_pct", 1.0),
            ("pnl_total", 5.0),
            ("herfindahl_index", 0.05),
            ("risk_score", 2.0),
        ]
        numbers = [float(x) for x in re.findall(r"-?\d+(?:[\.,]\d+)?", text.replace(",", "."))]
        if not numbers:
            return 0.0
        hits = 0
        for key, tol in numeric_keys:
            target = compact.get(key)
            if target is None:
                continue
            if any(_within(n, float(target), tol) for n in numbers):
                hits += 1
        return hits / float(len(numeric_keys))

    def _format_compliance(self, text: str) -> float:
        hits = sum(1 for section in REPORT_SECTIONS if section in text)
        return hits / float(len(REPORT_SECTIONS))


def _within(value: float, target: float, tolerance: float) -> bool:
    if target == 0:
        return abs(value - target) <= max(tolerance, 0.5)
    rel = abs(value - target) / max(abs(target), 1e-9)
    return rel <= 0.05 or abs(value - target) <= tolerance


def report_to_dict(report: EvalReport) -> dict:
    return asdict(report)
