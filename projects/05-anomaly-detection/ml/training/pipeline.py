from __future__ import annotations

import json
import logging
from dataclasses import dataclass, field
from pathlib import Path

import clickhouse_connect
import numpy as np
import pandas as pd

from config import MLConfig
from models import AutoencoderModel, IForestModel
from models.autoencoder import AutoencoderParams
from models.iforest import IForestParams

log = logging.getLogger(__name__)


FEATURE_COLUMNS: list[str] = [
    "amount",
    "avg_amount_1h",
    "avg_amount_24h",
    "unique_counterparties_24h",
    "z_score",
    "time_since_last_tx",
    "night_flag",
    "frequency_score",
]


@dataclass
class TrainingResult:
    rows: int = 0
    iforest_metrics: dict = field(default_factory=dict)
    autoencoder_metrics: dict = field(default_factory=dict)
    threshold: float = 0.0


class TrainingPipeline:
    def __init__(self, cfg: MLConfig) -> None:
        self.cfg = cfg

    def run(self, csv_path: str | None = None) -> TrainingResult:
        df = self._load_data(csv_path)
        if df.empty:
            raise RuntimeError("no training features available")

        X = self._prepare(df)
        log.info("training on %d rows", len(X))

        iforest = IForestModel(IForestParams(
            n_estimators=self.cfg.iforest_estimators,
            contamination=self.cfg.iforest_contamination,
            random_state=self.cfg.random_state,
        ))
        if_metrics = iforest.fit(X)

        flags = iforest.predict(X)
        X_normal = X[flags == 1]
        log.info("normal rows for autoencoder: %d", len(X_normal))
        if len(X_normal) < 200:
            X_normal = X

        ae = AutoencoderModel(AutoencoderParams(
            input_dim=X.shape[1],
            epochs=self.cfg.autoencoder_epochs,
            batch_size=self.cfg.autoencoder_batch,
            lr=self.cfg.autoencoder_lr,
            patience=self.cfg.autoencoder_patience,
            threshold_percentile=self.cfg.score_threshold_percentile,
            random_state=self.cfg.random_state,
        ))
        ae_metrics = ae.fit(X_normal)

        out = Path(self.cfg.model_dir)
        out.mkdir(parents=True, exist_ok=True)
        iforest.save(out)
        ae.save(out)

        result = TrainingResult(
            rows=int(len(X)),
            iforest_metrics=if_metrics,
            autoencoder_metrics=ae_metrics,
            threshold=ae.threshold,
        )
        (out / "training_result.json").write_text(
            json.dumps(self._serialize(result), indent=2, ensure_ascii=False),
            encoding="utf-8",
        )
        return result

    def evaluate(self, X: np.ndarray) -> dict:
        out = Path(self.cfg.model_dir)
        iforest = IForestModel()
        iforest.load(out)
        ae = AutoencoderModel()
        ae.load(out)

        flags = iforest.predict(X)
        errors = ae.predict(X)
        above = errors > ae.threshold
        confirmed = (flags == -1) & above
        return {
            "total": int(len(X)),
            "iforest_flagged": int((flags == -1).sum()),
            "ae_above_threshold": int(above.sum()),
            "confirmed_anomalies": int(confirmed.sum()),
            "threshold": ae.threshold,
            "mean_error": float(errors.mean()),
        }

    def _load_data(self, csv_path: str | None) -> pd.DataFrame:
        if csv_path:
            df = pd.read_csv(csv_path)
            log.info("loaded %d rows from csv", len(df))
            return df
        try:
            client = clickhouse_connect.get_client(
                host=self.cfg.clickhouse_host,
                port=self.cfg.clickhouse_port,
                username=self.cfg.clickhouse_user,
                password=self.cfg.clickhouse_password,
                database=self.cfg.clickhouse_db,
            )
        except Exception as e:
            raise RuntimeError(f"clickhouse connect failed: {e}")
        try:
            df = client.query_df(
                "SELECT " + ", ".join(FEATURE_COLUMNS) +
                " FROM anomalies.features WHERE ts >= now() - INTERVAL 7 DAY LIMIT 500000"
            )
            log.info("loaded %d rows from clickhouse", len(df))
            return df
        finally:
            client.close()

    def _prepare(self, df: pd.DataFrame) -> np.ndarray:
        for col in FEATURE_COLUMNS:
            if col not in df.columns:
                df[col] = 0.0
        df = df[FEATURE_COLUMNS].replace([np.inf, -np.inf], np.nan).fillna(0.0)
        for col in df.columns:
            lo, hi = df[col].quantile([0.001, 0.999])
            df[col] = df[col].clip(lo, hi)
        return df.to_numpy(dtype=np.float32)

    def _serialize(self, r: TrainingResult) -> dict:
        return {
            "rows": r.rows,
            "iforest_metrics": r.iforest_metrics,
            "autoencoder_metrics": r.autoencoder_metrics,
            "threshold": r.threshold,
        }
