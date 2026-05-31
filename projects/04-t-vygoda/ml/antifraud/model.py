from __future__ import annotations

import logging
from dataclasses import dataclass, field
from pathlib import Path

import joblib
import numpy as np
import pandas as pd
import xgboost as xgb
from sklearn.ensemble import IsolationForest

from antifraud.features import FRAUD_FEATURE_COLUMNS

log = logging.getLogger(__name__)


@dataclass
class FraudPrediction:
    user_id: int
    score: float
    is_fraud: bool
    reasons: list[str] = field(default_factory=list)


@dataclass
class DetectorParams:
    contamination: float = 0.05
    random_state: int = 42
    iso_estimators: int = 200
    xgb_estimators: int = 300


class FraudDetector:
    def __init__(self, params: DetectorParams | None = None) -> None:
        self.params = params or DetectorParams()
        self.iso: IsolationForest | None = None
        self.clf: xgb.XGBClassifier | None = None
        self.feature_columns: list[str] = list(FRAUD_FEATURE_COLUMNS)

    def fit(self, features_df: pd.DataFrame, labels: np.ndarray | None = None) -> dict[str, float]:
        if features_df.empty:
            raise ValueError("empty features for fraud detector")
        X = features_df[self.feature_columns].to_numpy(dtype=np.float64)
        self.iso = IsolationForest(
            n_estimators=self.params.iso_estimators,
            contamination=self.params.contamination,
            random_state=self.params.random_state,
            n_jobs=-1,
        )
        self.iso.fit(X)

        metrics: dict[str, float] = {"iso_trained": 1.0}
        if labels is not None:
            self.clf = xgb.XGBClassifier(
                n_estimators=self.params.xgb_estimators,
                max_depth=4,
                learning_rate=0.1,
                random_state=self.params.random_state,
                tree_method="hist",
                eval_metric="logloss",
            )
            self.clf.fit(X, labels)
            metrics["supervised"] = 1.0
        return metrics

    def predict(self, features_df: pd.DataFrame) -> list[FraudPrediction]:
        self._require_trained()
        X = features_df[self.feature_columns].to_numpy(dtype=np.float64)

        iso_scores = -self.iso.score_samples(X)
        iso_flags = self.iso.predict(X) == -1

        if self.clf is not None:
            sup_probs = self.clf.predict_proba(X)[:, 1]
            scores = 0.5 * iso_scores + 0.5 * sup_probs
            flags = (sup_probs > 0.5) | iso_flags
        else:
            scores = iso_scores
            flags = iso_flags

        out: list[FraudPrediction] = []
        for idx, user_id in enumerate(features_df.index):
            reasons = self._reasons(features_df.iloc[idx], iso_flags[idx])
            out.append(FraudPrediction(
                user_id=int(user_id),
                score=float(scores[idx]),
                is_fraud=bool(flags[idx]),
                reasons=reasons,
            ))
        return out

    def _reasons(self, row: pd.Series, iso_flag: bool) -> list[str]:
        reasons: list[str] = []
        if row["degree"] > 20:
            reasons.append("аномально много прямых рефералов")
        if row["subtree_size"] > 200:
            reasons.append("чрезмерно большое поддерево")
        if 0 < row["avg_time_between_referrals"] < 60:
            reasons.append("высокая частота приглашений")
        if row["same_ip_ratio"] > 0.5:
            reasons.append("много рефералов с одного IP")
        if row["conversion_rate"] < 0.05 and row["degree"] >= 10:
            reasons.append("приглашенные не покупают")
        if iso_flag and not reasons:
            reasons.append("выброс по совокупности признаков")
        return reasons

    def save(self, dir_path: str | Path) -> None:
        self._require_trained()
        dir_path = Path(dir_path)
        dir_path.mkdir(parents=True, exist_ok=True)
        joblib.dump(self.iso, dir_path / "fraud_iso.joblib")
        if self.clf is not None:
            joblib.dump(self.clf, dir_path / "fraud_clf.joblib")
        joblib.dump({
            "params": self.params,
            "feature_columns": self.feature_columns,
        }, dir_path / "fraud_meta.joblib")

    def load(self, dir_path: str | Path) -> None:
        dir_path = Path(dir_path)
        self.iso = joblib.load(dir_path / "fraud_iso.joblib")
        clf_path = dir_path / "fraud_clf.joblib"
        if clf_path.exists():
            self.clf = joblib.load(clf_path)
        meta = joblib.load(dir_path / "fraud_meta.joblib")
        self.params = meta.get("params", self.params)
        self.feature_columns = meta.get("feature_columns", self.feature_columns)

    def _require_trained(self) -> None:
        if self.iso is None:
            raise RuntimeError("fraud detector is not trained")
