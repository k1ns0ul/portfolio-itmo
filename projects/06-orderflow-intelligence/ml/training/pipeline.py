from __future__ import annotations

import json
import logging
from dataclasses import asdict, dataclass, field
from pathlib import Path

import numpy as np
import pandas as pd
from sklearn.metrics import accuracy_score, confusion_matrix, f1_score

from config import MLConfig
from db import ClickHouseDB
from ensemble import CLASS_NAMES, Ensemble
from models import DirectionLSTM, DirectionXGB
from models.lstm_model import LSTM_FEATURE_COLUMNS, LSTMParams
from models.xgb_model import XGB_FEATURE_COLUMNS, XGBParams

log = logging.getLogger(__name__)


LABEL_UP = 0
LABEL_DOWN = 1
LABEL_FLAT = 2


@dataclass
class SplitMetrics:
    accuracy: float = 0.0
    f1_macro: float = 0.0
    confusion: list[list[int]] = field(default_factory=list)


@dataclass
class TrainingResult:
    rows: int
    xgb_metrics: SplitMetrics
    lstm_metrics: SplitMetrics
    ensemble_metrics: SplitMetrics
    alpha: float


class TrainingPipeline:
    def __init__(self, cfg: MLConfig, db: ClickHouseDB | None = None) -> None:
        self.cfg = cfg
        self.db = db or ClickHouseDB(cfg)
        self.xgb = DirectionXGB(params=XGBParams(
            n_estimators=cfg.xgb_estimators,
            max_depth=cfg.xgb_max_depth,
            learning_rate=cfg.xgb_lr,
            random_state=cfg.random_state,
        ))
        self.lstm = DirectionLSTM(params=LSTMParams(
            seq_len=cfg.lstm_seq_len,
            hidden=cfg.lstm_hidden,
            layers=cfg.lstm_layers,
            dropout=cfg.lstm_dropout,
            epochs=cfg.lstm_epochs,
            batch_size=cfg.lstm_batch,
            lr=cfg.lstm_lr,
            patience=cfg.lstm_patience,
            random_state=cfg.random_state,
        ))
        self.ensemble = Ensemble(min_confidence=cfg.min_confidence)

    def run(self) -> TrainingResult:
        raw = self.db.fetch_features(interval_sec=self.cfg.interval_sec, limit=self.cfg.history_limit)
        if raw.empty:
            raise RuntimeError("no feature windows in clickhouse")
        labeled = self._make_labels(raw)
        if labeled.empty:
            raise RuntimeError("no labeled rows after target horizon")
        with_lags = self._add_lags(labeled)

        log.info("training rows=%d", len(with_lags))

        n = len(with_lags)
        train_end = int(n * 0.70)
        val_end = train_end + int(n * 0.15)
        train = with_lags.iloc[:train_end]
        val = with_lags.iloc[train_end:val_end]
        test = with_lags.iloc[val_end:]

        self.xgb.fit(train, train["target"].to_numpy(dtype=np.int64))
        xgb_val_probs = self.xgb.predict_proba(val)
        xgb_test_probs = self.xgb.predict_proba(test)

        seq_train, lbl_train = self._make_sequences(train, self.cfg.lstm_seq_len)
        seq_val, lbl_val = self._make_sequences(val, self.cfg.lstm_seq_len)
        seq_test, lbl_test = self._make_sequences(test, self.cfg.lstm_seq_len)
        if seq_train.size == 0:
            raise RuntimeError("not enough rows for lstm sequences")
        self.lstm.fit(seq_train, lbl_train)
        lstm_val_probs = self.lstm.predict_proba(seq_val)
        lstm_test_probs = self.lstm.predict_proba(seq_test)

        align_xgb_val = xgb_val_probs[self.cfg.lstm_seq_len - 1:]
        align_xgb_test = xgb_test_probs[self.cfg.lstm_seq_len - 1:]
        align_y_val = lbl_val
        align_y_test = lbl_test

        self.ensemble.fit_alpha(align_xgb_val, lstm_val_probs, align_y_val)

        result = TrainingResult(
            rows=int(n),
            xgb_metrics=self._eval(xgb_test_probs, test["target"].to_numpy(dtype=np.int64)),
            lstm_metrics=self._eval(lstm_test_probs, lbl_test),
            ensemble_metrics=self._eval(self.ensemble.blend(align_xgb_test, lstm_test_probs), align_y_test),
            alpha=self.ensemble.alpha,
        )
        self.save_all(result)
        return result

    def _make_labels(self, df: pd.DataFrame) -> pd.DataFrame:
        df = df.sort_values(["pair", "window_end"], kind="stable").reset_index(drop=True)
        df["window_end"] = pd.to_datetime(df["window_end"], utc=True)
        target_rows: list[int] = []
        df["target"] = -1
        horizon = pd.Timedelta(seconds=self.cfg.target_horizon_sec)
        threshold = self.cfg.move_threshold
        for pair, group in df.groupby("pair", sort=False):
            indexes = group.index.tolist()
            for i, idx in enumerate(indexes):
                row = group.loc[idx]
                future_time = row["window_end"] + horizon
                future_rows = group.loc[indexes[i:]]
                future_rows = future_rows[future_rows["window_end"] >= future_time]
                if future_rows.empty:
                    continue
                future_price = float(future_rows.iloc[0]["price_close"])
                current_price = float(row["price_close"])
                if current_price <= 0:
                    continue
                change = (future_price - current_price) / current_price
                if change > threshold:
                    label = LABEL_UP
                elif change < -threshold:
                    label = LABEL_DOWN
                else:
                    label = LABEL_FLAT
                df.at[idx, "target"] = label
                target_rows.append(idx)
        return df.loc[target_rows].copy()

    def _add_lags(self, df: pd.DataFrame) -> pd.DataFrame:
        df = df.copy()
        df["ofi_delta"] = df.groupby("pair")["ofi"].diff().fillna(0.0)
        df["ofi_lag1"] = df.groupby("pair")["ofi"].shift(1).fillna(0.0)
        df["ofi_lag2"] = df.groupby("pair")["ofi"].shift(2).fillna(0.0)
        df["vpin_lag1"] = df.groupby("pair")["vpin"].shift(1).fillna(0.0)
        for col in XGB_FEATURE_COLUMNS:
            if col not in df.columns:
                df[col] = 0.0
        return df

    def _make_sequences(self, df: pd.DataFrame, seq_len: int) -> tuple[np.ndarray, np.ndarray]:
        sequences: list[np.ndarray] = []
        labels: list[int] = []
        for _, group in df.groupby("pair", sort=False):
            arr = group[LSTM_FEATURE_COLUMNS].to_numpy(dtype=np.float32)
            targets = group["target"].to_numpy(dtype=np.int64)
            for i in range(seq_len - 1, len(group)):
                window = arr[i - seq_len + 1: i + 1]
                if window.shape[0] != seq_len:
                    continue
                sequences.append(window)
                labels.append(int(targets[i]))
        if not sequences:
            return np.zeros((0, seq_len, len(LSTM_FEATURE_COLUMNS)), dtype=np.float32), np.zeros(0, dtype=np.int64)
        return np.stack(sequences), np.array(labels, dtype=np.int64)

    def _eval(self, probs: np.ndarray, y_true: np.ndarray) -> SplitMetrics:
        if probs.size == 0 or y_true.size == 0:
            return SplitMetrics()
        preds = probs.argmax(axis=1)
        return SplitMetrics(
            accuracy=float(accuracy_score(y_true, preds)),
            f1_macro=float(f1_score(y_true, preds, average="macro", zero_division=0)),
            confusion=confusion_matrix(y_true, preds, labels=[0, 1, 2]).tolist(),
        )

    def save_all(self, result: TrainingResult) -> None:
        out = Path(self.cfg.model_dir)
        out.mkdir(parents=True, exist_ok=True)
        self.xgb.save(out)
        self.lstm.save(out)
        self.ensemble.save(out)
        (out / "training_result.json").write_text(
            json.dumps(self._serialize(result), indent=2, ensure_ascii=False),
            encoding="utf-8",
        )

    def _serialize(self, r: TrainingResult) -> dict:
        return {
            "rows": r.rows,
            "alpha": r.alpha,
            "class_order": CLASS_NAMES,
            "xgb_metrics": asdict(r.xgb_metrics),
            "lstm_metrics": asdict(r.lstm_metrics),
            "ensemble_metrics": asdict(r.ensemble_metrics),
        }
