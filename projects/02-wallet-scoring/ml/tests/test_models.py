from __future__ import annotations

import numpy as np
import pandas as pd

from models import WalletNeuralModel, WalletXGBModel
from models.neural_model import NeuralParams
from models.xgboost_model import XGBParams


def _synthetic_dataset(n: int = 600, seed: int = 0) -> tuple[pd.DataFrame, np.ndarray, np.ndarray]:
    rng = np.random.default_rng(seed)
    X = pd.DataFrame({
        "f0": rng.normal(size=n),
        "f1": rng.normal(size=n),
        "f2": rng.normal(size=n),
        "f3": rng.normal(size=n),
        "f4": rng.normal(size=n),
    })
    logits = 0.6 * X["f0"] + 0.3 * X["f1"] - 0.4 * X["f2"]
    score = (50 + 20 * logits).clip(0, 100).to_numpy(dtype=np.float32)
    label = np.where(score >= 60, 0, np.where(score >= 35, 1, 2)).astype(np.int64)
    return X, score, label


def test_xgb_train_and_predict():
    X, ys, yl = _synthetic_dataset(400)
    model = WalletXGBModel(params=XGBParams(n_estimators=50, max_depth=4))
    metrics = model.train(X, ys, yl)
    scores, labels, probs = model.predict(X)
    assert scores.shape == (len(X),)
    assert labels.shape == (len(X),)
    assert probs.shape == (len(X), 3)
    assert 0 <= scores.min() and scores.max() <= 100
    assert metrics["classifier_best_score"] > 0.3


def test_xgb_feature_importance_matches_columns():
    X, ys, yl = _synthetic_dataset(300)
    model = WalletXGBModel(params=XGBParams(n_estimators=30, max_depth=3))
    model.train(X, ys, yl)
    importance = model.feature_importance()
    assert set(importance) == set(X.columns)


def test_neural_train_and_predict_smoke():
    X, ys, yl = _synthetic_dataset(400, seed=1)
    model = WalletNeuralModel(params=NeuralParams(epochs=3, batch_size=64, lr=1e-2,
                                                  patience=3, num_workers=0))
    model.train(X, ys, yl, eval_set=(X.iloc[:80], ys[:80], yl[:80]))
    scores, labels, probs = model.predict(X)
    assert scores.shape == (len(X),)
    assert labels.shape == (len(X),)
    assert probs.shape == (len(X), 3)
    assert np.allclose(probs.sum(axis=1), 1.0, atol=1e-4)


def test_neural_save_load_roundtrip(tmp_path):
    X, ys, yl = _synthetic_dataset(200, seed=2)
    model = WalletNeuralModel(params=NeuralParams(epochs=2, batch_size=64, lr=1e-2,
                                                  patience=2, num_workers=0))
    model.train(X, ys, yl)
    path = tmp_path / "nn.pt"
    model.save(path)

    loaded = WalletNeuralModel(params=NeuralParams(num_workers=0))
    loaded.load(path)
    s1, _, _ = model.predict(X)
    s2, _, _ = loaded.predict(X)
    assert np.allclose(s1, s2, atol=1e-4)
