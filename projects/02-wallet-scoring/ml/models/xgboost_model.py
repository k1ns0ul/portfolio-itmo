from __future__ import annotations

import logging
from dataclasses import dataclass, field
from pathlib import Path

import joblib
import numpy as np
import pandas as pd
import xgboost as xgb

log = logging.getLogger(__name__)


@dataclass
class XGBParams:
    max_depth: int = 6
    n_estimators: int = 300
    learning_rate: float = 0.1
    subsample: float = 0.8
    colsample_bytree: float = 0.8
    reg_lambda: float = 1.0
    random_state: int = 42
    tree_method: str = "hist"


@dataclass
class WalletXGBModel:
    params: XGBParams = field(default_factory=XGBParams)
    regressor: xgb.XGBRegressor | None = None
    classifier: xgb.XGBClassifier | None = None
    feature_names: list[str] | None = None

    def train(
        self,
        X: pd.DataFrame,
        y_score: np.ndarray,
        y_label: np.ndarray,
        eval_set: tuple[pd.DataFrame, np.ndarray, np.ndarray] | None = None,
    ) -> dict[str, float]:
        self.feature_names = list(X.columns)
        reg = xgb.XGBRegressor(
            objective="reg:squarederror",
            n_estimators=self.params.n_estimators,
            max_depth=self.params.max_depth,
            learning_rate=self.params.learning_rate,
            subsample=self.params.subsample,
            colsample_bytree=self.params.colsample_bytree,
            reg_lambda=self.params.reg_lambda,
            random_state=self.params.random_state,
            tree_method=self.params.tree_method,
        )
        clf = xgb.XGBClassifier(
            objective="multi:softprob",
            num_class=3,
            n_estimators=self.params.n_estimators,
            max_depth=self.params.max_depth,
            learning_rate=self.params.learning_rate,
            subsample=self.params.subsample,
            colsample_bytree=self.params.colsample_bytree,
            reg_lambda=self.params.reg_lambda,
            random_state=self.params.random_state,
            tree_method=self.params.tree_method,
            eval_metric="mlogloss",
        )

        reg_kwargs: dict = {}
        clf_kwargs: dict = {}
        if eval_set is not None:
            Xv, ys_val, yl_val = eval_set
            reg_kwargs["eval_set"] = [(Xv, ys_val)]
            reg_kwargs["verbose"] = False
            clf_kwargs["eval_set"] = [(Xv, yl_val)]
            clf_kwargs["verbose"] = False

        reg.fit(X, y_score, **reg_kwargs)
        clf.fit(X, y_label, **clf_kwargs)

        self.regressor = reg
        self.classifier = clf

        metrics = {
            "regressor_best_score": float(reg.score(X, y_score)),
            "classifier_best_score": float(clf.score(X, y_label)),
        }
        log.info("xgb trained; r2=%.4f, acc=%.4f",
                 metrics["regressor_best_score"], metrics["classifier_best_score"])
        return metrics

    def predict(self, X: pd.DataFrame) -> tuple[np.ndarray, np.ndarray, np.ndarray]:
        self._require_trained()
        X_aligned = self._align(X)
        scores = self.regressor.predict(X_aligned)
        probs = self.classifier.predict_proba(X_aligned)
        labels = probs.argmax(axis=1)
        return np.clip(scores, 0.0, 100.0), labels.astype(int), probs

    def feature_importance(self) -> dict[str, float]:
        self._require_trained()
        score_imp = self.regressor.feature_importances_
        clf_imp = self.classifier.feature_importances_
        combined = (score_imp + clf_imp) / 2.0
        return {name: float(v) for name, v in zip(self.feature_names, combined)}

    def save(self, path: str | Path) -> None:
        self._require_trained()
        path = Path(path)
        path.parent.mkdir(parents=True, exist_ok=True)
        joblib.dump(
            {
                "regressor": self.regressor,
                "classifier": self.classifier,
                "feature_names": self.feature_names,
                "params": self.params,
            },
            path,
        )
        log.info("xgb saved to %s", path)

    def load(self, path: str | Path) -> None:
        blob = joblib.load(path)
        self.regressor = blob["regressor"]
        self.classifier = blob["classifier"]
        self.feature_names = blob["feature_names"]
        self.params = blob.get("params", self.params)
        log.info("xgb loaded from %s", path)

    def _align(self, X: pd.DataFrame) -> pd.DataFrame:
        if self.feature_names is None:
            return X
        missing = [c for c in self.feature_names if c not in X.columns]
        if missing:
            for c in missing:
                X[c] = 0.0
        return X[self.feature_names]

    def _require_trained(self) -> None:
        if self.regressor is None or self.classifier is None:
            raise RuntimeError("xgb model is not trained")
