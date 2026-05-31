from __future__ import annotations

import logging
from dataclasses import dataclass
from pathlib import Path

import joblib
import numpy as np
from sklearn.ensemble import IsolationForest
from sklearn.preprocessing import StandardScaler

log = logging.getLogger(__name__)


@dataclass
class IForestParams:
    n_estimators: int = 200
    contamination: float = 0.01
    max_features: float = 0.8
    random_state: int = 42


class IForestModel:
    def __init__(self, params: IForestParams | None = None) -> None:
        self.params = params or IForestParams()
        self.scaler: StandardScaler | None = None
        self.model: IsolationForest | None = None

    def fit(self, X: np.ndarray) -> dict[str, float]:
        if X.size == 0:
            raise ValueError("empty training set")
        self.scaler = StandardScaler()
        Xs = self.scaler.fit_transform(X)
        self.model = IsolationForest(
            n_estimators=self.params.n_estimators,
            contamination=self.params.contamination,
            max_features=self.params.max_features,
            random_state=self.params.random_state,
            n_jobs=-1,
        )
        self.model.fit(Xs)
        flags = self.model.predict(Xs)
        anomaly_rate = float((flags == -1).mean())
        log.info("iforest fit done; anomaly_rate=%.4f", anomaly_rate)
        return {"anomaly_rate": anomaly_rate, "rows": int(len(X))}

    def predict(self, X: np.ndarray) -> np.ndarray:
        self._require_trained()
        Xs = self.scaler.transform(X)
        return self.model.predict(Xs)

    def scores(self, X: np.ndarray) -> np.ndarray:
        self._require_trained()
        Xs = self.scaler.transform(X)
        return -self.model.score_samples(Xs)

    def save(self, dir_path: str | Path) -> None:
        self._require_trained()
        dir_path = Path(dir_path)
        dir_path.mkdir(parents=True, exist_ok=True)
        joblib.dump({"model": self.model, "scaler": self.scaler, "params": self.params}, dir_path / "iforest.joblib")

    def load(self, dir_path: str | Path) -> None:
        blob = joblib.load(Path(dir_path) / "iforest.joblib")
        self.model = blob["model"]
        self.scaler = blob["scaler"]
        self.params = blob.get("params", self.params)

    def _require_trained(self) -> None:
        if self.model is None or self.scaler is None:
            raise RuntimeError("iforest is not trained")
