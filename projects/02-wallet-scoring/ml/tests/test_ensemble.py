from __future__ import annotations

import numpy as np
import pandas as pd

from models import EnsembleModel, WalletNeuralModel, WalletXGBModel
from models.neural_model import NeuralParams
from models.xgboost_model import XGBParams


def _dataset(n: int = 400, seed: int = 7) -> tuple[pd.DataFrame, np.ndarray, np.ndarray]:
    rng = np.random.default_rng(seed)
    X = pd.DataFrame({
        "a": rng.normal(size=n),
        "b": rng.normal(size=n),
        "c": rng.normal(size=n),
        "d": rng.normal(size=n),
    })
    score = (50 + 25 * X["a"] - 15 * X["b"]).clip(0, 100).to_numpy(dtype=np.float32)
    label = np.where(score >= 60, 0, np.where(score >= 35, 1, 2)).astype(np.int64)
    return X, score, label


def _fit_models() -> tuple[EnsembleModel, pd.DataFrame, np.ndarray]:
    X, ys, yl = _dataset(300)
    ensemble = EnsembleModel(
        xgb=WalletXGBModel(params=XGBParams(n_estimators=40, max_depth=3)),
        nn=WalletNeuralModel(params=NeuralParams(epochs=3, batch_size=64, lr=1e-2,
                                                 patience=3, num_workers=0)),
    )
    ensemble.xgb.train(X, ys, yl)
    ensemble.nn.train(X, ys, yl)
    return ensemble, X, ys


def test_alpha_chosen_in_range():
    ensemble, X, ys = _fit_models()
    alpha = ensemble.fit_alpha(X.iloc[:100], ys[:100])
    assert 0.0 <= alpha <= 1.0


def test_predict_frame_columns():
    ensemble, X, ys = _fit_models()
    ensemble.fit_alpha(X.iloc[:80], ys[:80])
    df = ensemble.predict_frame(X.iloc[:50])
    for col in ("score", "label", "category", "confidence", "xgb_score", "nn_score"):
        assert col in df.columns
    assert df["category"].isin({"legit", "suspicious", "scam"}).all()


def test_predict_one():
    ensemble, X, ys = _fit_models()
    ensemble.fit_alpha(X.iloc[:80], ys[:80])
    row = X.iloc[0]
    pred = ensemble.predict_one("WALLET-X", row)
    assert pred.wallet == "WALLET-X"
    assert 0.0 <= pred.score <= 100.0
    assert pred.label in (0, 1, 2)
    assert 0.0 <= pred.confidence <= 1.0
