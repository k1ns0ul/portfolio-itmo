from __future__ import annotations

import logging
from dataclasses import dataclass, field
from pathlib import Path

import joblib
import numpy as np
import pandas as pd
import xgboost as xgb
from sklearn.preprocessing import StandardScaler

log = logging.getLogger(__name__)


XGB_FEATURE_COLUMNS: list[str] = [
    "ofi",
    "ofi_delta",
    "vpin",
    "price_impact",
    "avg_swap_size",
    "buy_ratio",
    "cumulative_volume",
    "price_range",
    "swap_count",
    "ofi_lag1",
    "ofi_lag2",
    "vpin_lag1",
]


@dataclass
class XGBParams:
    n_estimators: int = 200
    max_depth: int = 5
    learning_rate: float = 0.1
    subsample: float = 0.8
    colsample_bytree: float = 0.8
    random_state: int = 42


@dataclass
class DirectionXGB:
    params: XGBParams = field(default_factory=XGBParams)
    model: xgb.XGBClassifier | None = None
    scaler: StandardScaler | None = None
    feature_columns: list[str] = field(default_factory=lambda: list(XGB_FEATURE_COLUMNS))

    def fit(self, X: pd.DataFrame, y: np.ndarray) -> dict[str, float]:
        if X.empty:
            raise ValueError("empty training set")
        self.scaler = StandardScaler()
        Xs = self.scaler.fit_transform(X[self.feature_columns].to_numpy(dtype=np.float32))
        self.model = xgb.XGBClassifier(
            objective="multi:softprob",
            num_class=3,
            n_estimators=self.params.n_estimators,
            max_depth=self.params.max_depth,
            learning_rate=self.params.learning_rate,
            subsample=self.params.subsample,
            colsample_bytree=self.params.colsample_bytree,
            random_state=self.params.random_state,
            tree_method="hist",
            eval_metric="mlogloss",
        )
        self.model.fit(Xs, y)
        acc = float((self.model.predict(Xs) == y).mean())
        log.info("xgb fit done; train_accuracy=%.4f rows=%d", acc, len(X))
        return {"train_accuracy": acc, "rows": int(len(X))}

    def predict_proba(self, X: pd.DataFrame) -> np.ndarray:
        self._require_trained()
        Xs = self.scaler.transform(X[self.feature_columns].to_numpy(dtype=np.float32))
        return self.model.predict_proba(Xs)

    def save(self, dir_path: str | Path) -> None:
        self._require_trained()
        dir_path = Path(dir_path)
        dir_path.mkdir(parents=True, exist_ok=True)
        joblib.dump({
            "model": self.model,
            "scaler": self.scaler,
            "params": self.params,
            "feature_columns": self.feature_columns,
        }, dir_path / "xgb.joblib")

    def load(self, dir_path: str | Path) -> None:
        blob = joblib.load(Path(dir_path) / "xgb.joblib")
        self.model = blob["model"]
        self.scaler = blob["scaler"]
        self.params = blob.get("params", self.params)
        self.feature_columns = blob.get("feature_columns", self.feature_columns)

    def _require_trained(self) -> None:
        if self.model is None or self.scaler is None:
            raise RuntimeError("xgb is not trained")
