from __future__ import annotations

import json
import logging
from pathlib import Path
from typing import TextIO

log = logging.getLogger(__name__)


def write_report(model_dir: str | Path, dst: TextIO | str | Path | None = None) -> str:
    model_dir = Path(model_dir)
    payload = json.loads((model_dir / "training_result.json").read_text(encoding="utf-8"))

    lines: list[str] = []
    lines.append("Wallet scoring training report")
    lines.append("=" * 60)
    lines.append(f"Finished at:        {payload['finished_at']}")
    lines.append(f"Dataset size:       {payload['dataset_size']}")
    lines.append(f"Train duration:     {payload['train_duration_sec']:.1f}s")
    lines.append(f"Ensemble alpha:     {payload['alpha']:.2f}")
    lines.append("")
    lines.append("Class distribution (full set):")
    for k in sorted(payload["class_distribution"]):
        name = _CAT[int(k)]
        lines.append(f"  {k} ({name:<10}): {payload['class_distribution'][k]}")
    lines.append("")

    for section in ("xgb_metrics", "nn_metrics", "ensemble_metrics"):
        m = payload[section]
        lines.append(f"{section.replace('_', ' ').title()}:")
        lines.append(f"  RMSE:     {m['rmse']:.3f}")
        lines.append(f"  Accuracy: {m['accuracy']:.3f}")
        lines.append(f"  F1 macro: {m['f1_macro']:.3f}")
        lines.append("  Confusion matrix (rows=true, cols=pred, classes 0/1/2):")
        for row in m["confusion"]:
            lines.append("    " + " ".join(f"{c:>6}" for c in row))
        lines.append("")

    importance = payload["feature_importance"]
    top = sorted(importance.items(), key=lambda kv: kv[1], reverse=True)[:10]
    lines.append("Top 10 features by importance:")
    for name, val in top:
        lines.append(f"  {name:<24} {val:.4f}")
    lines.append("")

    text = "\n".join(lines)
    target = _resolve_dst(dst, model_dir)
    if target is not None:
        target.write_text(text, encoding="utf-8")
        log.info("report written to %s", target)
    return text


_CAT = {0: "legit", 1: "suspicious", 2: "scam"}


def _resolve_dst(dst, model_dir: Path) -> Path | None:
    if dst is None:
        return model_dir / "report.txt"
    if isinstance(dst, (str, Path)):
        return Path(dst)
    return None
