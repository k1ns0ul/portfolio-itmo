from __future__ import annotations

import json
import logging
import time
from dataclasses import asdict, dataclass, field
from datetime import datetime, timezone
from pathlib import Path

import joblib
import numpy as np
import pandas as pd
from sklearn.metrics import (accuracy_score, confusion_matrix, f1_score,
                             mean_squared_error)
from sklearn.model_selection import train_test_split
from sklearn.preprocessing import StandardScaler

from config import MLConfig
from db import ClickHouseDB
from features import FEATURE_COLUMNS, FeatureExtractor, HeuristicLabeler
from models import EnsembleModel, WalletNeuralModel, WalletXGBModel
from models.neural_model import NeuralParams
from models.xgboost_model import XGBParams

log = logging.getLogger(__name__)


@dataclass
class SplitMetrics:
    rmse: float = 0.0
    accuracy: float = 0.0
    f1_macro: float = 0.0
    confusion: list[list[int]] = field(default_factory=list)
    class_counts: dict[int, int] = field(default_factory=dict)


@dataclass
class TrainingResult:
    dataset_size: int
    class_distribution: dict[int, int]
    xgb_metrics: SplitMetrics
    nn_metrics: SplitMetrics
    ensemble_metrics: SplitMetrics
    feature_importance: dict[str, float]
    alpha: float
    train_duration_sec: float
    finished_at: str


class TrainingPipeline:
    def __init__(self, cfg: MLConfig, db: ClickHouseDB | None = None) -> None:
        self.cfg = cfg
        self.db = db or ClickHouseDB(cfg)
        self.extractor = FeatureExtractor()
        self.labeler = HeuristicLabeler()
        self.scaler = StandardScaler()
        self.ensemble = EnsembleModel(
            xgb=WalletXGBModel(params=XGBParams(
                max_depth=cfg.xgb_max_depth,
                n_estimators=cfg.xgb_estimators,
                learning_rate=cfg.xgb_lr,
                random_state=cfg.random_state,
            )),
            nn=WalletNeuralModel(params=NeuralParams(
                epochs=cfg.nn_epochs,
                batch_size=cfg.nn_batch_size,
                lr=cfg.nn_lr,
                patience=cfg.nn_patience,
            )),
        )

    def run(self) -> TrainingResult:
        start = time.time()
        raw = self.db.fetch_training_data()
        if raw.empty:
            raise RuntimeError("no training data in ClickHouse")
        features = self.extractor.extract(raw)
        labels = self.labeler.label(features, raw)

        X = self._preprocess(features)
        y_score = labels["target_score"].to_numpy(dtype=np.float32)
        y_label = labels["target_label"].to_numpy(dtype=np.int64)

        X_train, X_temp, ys_train, ys_temp, yl_train, yl_temp = train_test_split(
            X, y_score, y_label, test_size=0.30,
            random_state=self.cfg.random_state, stratify=y_label,
        )
        X_val, X_test, ys_val, ys_test, yl_val, yl_test = train_test_split(
            X_temp, ys_temp, yl_temp, test_size=0.50,
            random_state=self.cfg.random_state, stratify=yl_temp,
        )

        log.info("dataset sizes train=%d val=%d test=%d", len(X_train), len(X_val), len(X_test))

        self.ensemble.xgb.train(X_train, ys_train, yl_train,
                                eval_set=(X_val, ys_val, yl_val))
        self.ensemble.nn.train(X_train, ys_train, yl_train,
                               eval_set=(X_val, ys_val, yl_val))
        self.ensemble.fit_alpha(X_val, ys_val)

        xgb_metrics = self._eval(self.ensemble.xgb.predict(X_test), ys_test, yl_test)
        nn_metrics = self._eval(self.ensemble.nn.predict(X_test), ys_test, yl_test)
        ensemble_pred = self.ensemble.predict_frame(X_test)
        ensemble_metrics = self._eval_frame(ensemble_pred, ys_test, yl_test)

        result = TrainingResult(
            dataset_size=len(X),
            class_distribution=self._class_dist(y_label),
            xgb_metrics=xgb_metrics,
            nn_metrics=nn_metrics,
            ensemble_metrics=ensemble_metrics,
            feature_importance=self.ensemble.xgb.feature_importance(),
            alpha=self.ensemble.alpha,
            train_duration_sec=time.time() - start,
            finished_at=datetime.now(timezone.utc).isoformat(),
        )

        self.save_all(result)
        return result

    def _preprocess(self, features: pd.DataFrame) -> pd.DataFrame:
        df = features.copy()
        df = df.replace([np.inf, -np.inf], np.nan)
        df = df.fillna(df.median(numeric_only=True))
        df = df.fillna(0.0)

        for col in df.columns:
            lo, hi = df[col].quantile([0.01, 0.99])
            df[col] = df[col].clip(lo, hi)

        scaled = self.scaler.fit_transform(df.to_numpy(dtype=np.float32))
        return pd.DataFrame(scaled, columns=df.columns, index=df.index)

    def _eval(self, prediction, ys_true: np.ndarray, yl_true: np.ndarray) -> SplitMetrics:
        scores, labels, _ = prediction
        return SplitMetrics(
            rmse=float(np.sqrt(mean_squared_error(ys_true, scores))),
            accuracy=float(accuracy_score(yl_true, labels)),
            f1_macro=float(f1_score(yl_true, labels, average="macro", zero_division=0)),
            confusion=confusion_matrix(yl_true, labels, labels=[0, 1, 2]).tolist(),
            class_counts=self._class_dist(labels),
        )

    def _eval_frame(self, frame: pd.DataFrame, ys_true: np.ndarray, yl_true: np.ndarray) -> SplitMetrics:
        scores = frame["score"].to_numpy()
        labels = frame["label"].to_numpy()
        return SplitMetrics(
            rmse=float(np.sqrt(mean_squared_error(ys_true, scores))),
            accuracy=float(accuracy_score(yl_true, labels)),
            f1_macro=float(f1_score(yl_true, labels, average="macro", zero_division=0)),
            confusion=confusion_matrix(yl_true, labels, labels=[0, 1, 2]).tolist(),
            class_counts=self._class_dist(labels),
        )

    def _class_dist(self, labels: np.ndarray) -> dict[int, int]:
        unique, counts = np.unique(labels, return_counts=True)
        return {int(k): int(v) for k, v in zip(unique, counts)}

    def save_all(self, result: TrainingResult) -> None:
        out = Path(self.cfg.model_dir)
        out.mkdir(parents=True, exist_ok=True)
        self.ensemble.save(out)
        joblib.dump({"scaler": self.scaler, "feature_columns": FEATURE_COLUMNS}, out / "preproc.joblib")
        with (out / "training_result.json").open("w", encoding="utf-8") as f:
            json.dump(_serialize(result), f, indent=2)
        log.info("artifacts saved to %s", out)


def _serialize(result: TrainingResult) -> dict:
    return {
        "dataset_size": result.dataset_size,
        "class_distribution": result.class_distribution,
        "xgb_metrics": asdict(result.xgb_metrics),
        "nn_metrics": asdict(result.nn_metrics),
        "ensemble_metrics": asdict(result.ensemble_metrics),
        "feature_importance": result.feature_importance,
        "alpha": result.alpha,
        "train_duration_sec": result.train_duration_sec,
        "finished_at": result.finished_at,
    }


def load_serving(cfg: MLConfig) -> tuple[EnsembleModel, StandardScaler, list[str]]:
    out = Path(cfg.model_dir)
    ensemble = EnsembleModel()
    ensemble.load(out)
    blob = joblib.load(out / "preproc.joblib")
    return ensemble, blob["scaler"], blob["feature_columns"]
