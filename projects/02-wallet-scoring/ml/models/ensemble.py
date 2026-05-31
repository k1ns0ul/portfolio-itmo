from __future__ import annotations

import json
import logging
from dataclasses import asdict, dataclass, field
from pathlib import Path

import numpy as np
import pandas as pd

from models.neural_model import WalletNeuralModel
from models.xgboost_model import WalletXGBModel

log = logging.getLogger(__name__)

_LABEL_TO_CATEGORY = {0: "legit", 1: "suspicious", 2: "scam"}


@dataclass
class WalletPrediction:
    wallet: str
    score: float
    label: int
    category: str
    confidence: float
    xgb_score: float
    nn_score: float
    xgb_label: int
    nn_label: int


@dataclass
class EnsembleModel:
    xgb: WalletXGBModel = field(default_factory=WalletXGBModel)
    nn: WalletNeuralModel = field(default_factory=WalletNeuralModel)
    alpha: float = 0.5

    def fit_alpha(self, X_val: pd.DataFrame, y_score_val: np.ndarray) -> float:
        xgb_scores, _, _ = self.xgb.predict(X_val)
        nn_scores, _, _ = self.nn.predict(X_val)
        best_alpha = 0.5
        best_rmse = float("inf")
        for alpha in np.arange(0.0, 1.01, 0.05):
            blended = alpha * xgb_scores + (1.0 - alpha) * nn_scores
            rmse = float(np.sqrt(np.mean((blended - y_score_val) ** 2)))
            if rmse < best_rmse:
                best_rmse = rmse
                best_alpha = float(alpha)
        self.alpha = best_alpha
        log.info("ensemble alpha=%.2f rmse=%.4f", best_alpha, best_rmse)
        return best_alpha

    def predict_frame(self, X: pd.DataFrame) -> pd.DataFrame:
        xgb_scores, xgb_labels, xgb_probs = self.xgb.predict(X)
        nn_scores, nn_labels, nn_probs = self.nn.predict(X)

        blended_score = np.clip(self.alpha * xgb_scores + (1.0 - self.alpha) * nn_scores, 0.0, 100.0)

        xgb_conf = xgb_probs.max(axis=1)
        nn_conf = nn_probs.max(axis=1)

        agree = xgb_labels == nn_labels
        final_label = np.where(agree, xgb_labels, np.where(xgb_conf >= nn_conf, xgb_labels, nn_labels)).astype(int)
        final_conf = np.where(agree, (xgb_conf + nn_conf) / 2.0, np.maximum(xgb_conf, nn_conf))

        out = pd.DataFrame(
            {
                "score": blended_score,
                "label": final_label,
                "category": [_LABEL_TO_CATEGORY[i] for i in final_label],
                "confidence": final_conf,
                "xgb_score": xgb_scores,
                "nn_score": nn_scores,
                "xgb_label": xgb_labels,
                "nn_label": nn_labels,
            },
            index=X.index,
        )
        return out

    def predict_one(self, wallet: str, x: pd.Series) -> WalletPrediction:
        df = self.predict_frame(pd.DataFrame([x.values], columns=x.index, index=[wallet]))
        row = df.iloc[0]
        return WalletPrediction(
            wallet=wallet,
            score=float(row["score"]),
            label=int(row["label"]),
            category=str(row["category"]),
            confidence=float(row["confidence"]),
            xgb_score=float(row["xgb_score"]),
            nn_score=float(row["nn_score"]),
            xgb_label=int(row["xgb_label"]),
            nn_label=int(row["nn_label"]),
        )

    def save(self, dir_path: str | Path) -> None:
        dir_path = Path(dir_path)
        dir_path.mkdir(parents=True, exist_ok=True)
        self.xgb.save(dir_path / "xgb.joblib")
        self.nn.save(dir_path / "nn.pt")
        with (dir_path / "ensemble.json").open("w", encoding="utf-8") as f:
            json.dump({"alpha": self.alpha}, f)

    def load(self, dir_path: str | Path) -> None:
        dir_path = Path(dir_path)
        self.xgb.load(dir_path / "xgb.joblib")
        self.nn.load(dir_path / "nn.pt")
        with (dir_path / "ensemble.json").open("r", encoding="utf-8") as f:
            self.alpha = float(json.load(f)["alpha"])

    def params_snapshot(self) -> dict:
        return {
            "alpha": self.alpha,
            "xgb": asdict(self.xgb.params),
            "nn": self.nn.params.__dict__,
        }
