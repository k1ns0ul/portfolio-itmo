from __future__ import annotations

import logging
import signal
import time
from pathlib import Path

import numpy as np
import pandas as pd

from config import MLConfig
from db import ClickHouseDB
from ensemble import Ensemble
from models import DirectionLSTM, DirectionXGB
from models.lstm_model import LSTM_FEATURE_COLUMNS
from models.xgb_model import XGB_FEATURE_COLUMNS

log = logging.getLogger(__name__)


class PredictionService:
    def __init__(self, cfg: MLConfig, db: ClickHouseDB | None = None) -> None:
        self.cfg = cfg
        self.db = db or ClickHouseDB(cfg)
        self.xgb = DirectionXGB()
        self.lstm = DirectionLSTM()
        self.ensemble = Ensemble(min_confidence=cfg.min_confidence)
        self.stop_event = False
        self._install_signals()

    def setup(self) -> None:
        out = Path(self.cfg.model_dir)
        if not (out / "xgb.joblib").exists() or not (out / "lstm.pt").exists():
            raise RuntimeError(f"models not found in {out}; train first")
        self.xgb.load(out)
        self.lstm.load(out)
        self.ensemble.load(out)
        log.info("predictor ready (alpha=%.2f)", self.ensemble.alpha)

    def run_batch(self) -> int:
        pairs = self.db.list_pairs(self.cfg.interval_sec)
        if not pairs:
            return 0
        total = 0
        predictions: list[dict] = []
        for pair in pairs:
            pred = self._predict_pair(pair)
            if pred is not None:
                predictions.append(pred)
        if predictions:
            total = self.db.write_predictions(predictions)
            log.info("batch wrote %d predictions", total)
        return total

    def run_daemon(self) -> None:
        log.info("daemon mode interval=%ds", self.cfg.daemon_interval_sec)
        last = 0.0
        while not self.stop_event:
            now = time.time()
            if now - last >= self.cfg.daemon_interval_sec:
                try:
                    self.run_batch()
                except Exception as e:
                    log.exception("daemon tick failed: %s", e)
                last = now
            time.sleep(1.0)
        self.db.close()

    def _predict_pair(self, pair: str) -> dict | None:
        df = self.db.fetch_latest(pair, self.cfg.interval_sec, self.cfg.lstm_seq_len + 2)
        if df.empty or len(df) < self.cfg.lstm_seq_len:
            return None
        df = self._prepare(df)
        xgb_input = df.iloc[[-1]]
        xgb_probs = self.xgb.predict_proba(xgb_input)
        seq = df[LSTM_FEATURE_COLUMNS].to_numpy(dtype=np.float32)[-self.cfg.lstm_seq_len:][None, :, :]
        lstm_probs = self.lstm.predict_proba(seq)
        directions, blended = self.ensemble.predict(xgb_probs, lstm_probs)
        direction = directions[0]
        return {
            "pair": pair,
            "window_end": pd.Timestamp(df.iloc[-1]["window_end"]).to_pydatetime(),
            "direction": direction,
            "confidence": float(blended[0].max()),
            "xgb_prob": float(xgb_probs[0].max()),
            "lstm_prob": float(lstm_probs[0].max()),
        }

    def _prepare(self, df: pd.DataFrame) -> pd.DataFrame:
        df = df.copy()
        df["ofi_delta"] = df["ofi"].diff().fillna(0.0)
        df["ofi_lag1"] = df["ofi"].shift(1).fillna(0.0)
        df["ofi_lag2"] = df["ofi"].shift(2).fillna(0.0)
        df["vpin_lag1"] = df["vpin"].shift(1).fillna(0.0)
        for col in XGB_FEATURE_COLUMNS:
            if col not in df.columns:
                df[col] = 0.0
        return df

    def _install_signals(self) -> None:
        def stop(_signum, _frame):
            log.info("signal received")
            self.stop_event = True
        signal.signal(signal.SIGINT, stop)
        signal.signal(signal.SIGTERM, stop)
