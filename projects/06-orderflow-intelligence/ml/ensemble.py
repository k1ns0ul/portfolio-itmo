from __future__ import annotations

import json
import logging
from dataclasses import dataclass
from pathlib import Path

import numpy as np

log = logging.getLogger(__name__)


CLASS_NAMES = ["up", "down", "flat"]


@dataclass
class Ensemble:
    alpha: float = 0.5
    min_confidence: float = 0.4

    def blend(self, xgb_probs: np.ndarray, lstm_probs: np.ndarray) -> np.ndarray:
        return self.alpha * xgb_probs + (1.0 - self.alpha) * lstm_probs

    def predict(self, xgb_probs: np.ndarray, lstm_probs: np.ndarray) -> tuple[list[str], np.ndarray]:
        blended = self.blend(xgb_probs, lstm_probs)
        argmax = blended.argmax(axis=1)
        max_prob = blended.max(axis=1)
        directions: list[str] = []
        for i, idx in enumerate(argmax):
            if float(max_prob[i]) < self.min_confidence:
                directions.append("flat")
            else:
                directions.append(CLASS_NAMES[int(idx)])
        return directions, blended

    def fit_alpha(self, xgb_probs: np.ndarray, lstm_probs: np.ndarray, y_true: np.ndarray) -> float:
        best_alpha = 0.5
        best_acc = -1.0
        for alpha in np.arange(0.0, 1.01, 0.05):
            self.alpha = float(alpha)
            blended = self.blend(xgb_probs, lstm_probs)
            preds = blended.argmax(axis=1)
            acc = float((preds == y_true).mean())
            if acc > best_acc:
                best_acc = acc
                best_alpha = float(alpha)
        self.alpha = best_alpha
        log.info("ensemble alpha=%.2f acc=%.4f", best_alpha, best_acc)
        return best_alpha

    def save(self, dir_path: str | Path) -> None:
        dir_path = Path(dir_path)
        dir_path.mkdir(parents=True, exist_ok=True)
        (dir_path / "ensemble.json").write_text(
            json.dumps({"alpha": self.alpha, "min_confidence": self.min_confidence}, indent=2),
            encoding="utf-8",
        )

    def load(self, dir_path: str | Path) -> None:
        data = json.loads((Path(dir_path) / "ensemble.json").read_text(encoding="utf-8"))
        self.alpha = float(data.get("alpha", 0.5))
        self.min_confidence = float(data.get("min_confidence", self.min_confidence))
