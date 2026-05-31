from __future__ import annotations

import numpy as np
import pandas as pd

from models import DirectionXGB
from models.xgb_model import XGB_FEATURE_COLUMNS, XGBParams


def _synth(n: int = 600, seed: int = 0) -> tuple[pd.DataFrame, np.ndarray]:
    rng = np.random.default_rng(seed)
    rows = []
    labels = []
    for i in range(n):
        cls = i % 3
        if cls == 0:
            ofi = rng.normal(50, 5)
        elif cls == 1:
            ofi = rng.normal(-50, 5)
        else:
            ofi = rng.normal(0, 5)
        row = {col: 0.0 for col in XGB_FEATURE_COLUMNS}
        row["ofi"] = ofi
        row["vpin"] = abs(rng.normal(0.1, 0.05))
        row["price_impact"] = rng.normal(0, 0.01)
        row["avg_swap_size"] = abs(rng.normal(100, 20))
        row["buy_ratio"] = float(rng.uniform(0, 1))
        row["cumulative_volume"] = abs(rng.normal(1000, 200))
        row["price_range"] = abs(rng.normal(0.05, 0.02))
        row["swap_count"] = float(rng.integers(5, 50))
        rows.append(row)
        labels.append(cls)
    return pd.DataFrame(rows), np.array(labels, dtype=np.int64)


def test_fit_and_predict_shapes():
    X, y = _synth()
    model = DirectionXGB(params=XGBParams(n_estimators=50, max_depth=4))
    metrics = model.fit(X, y)
    assert metrics["rows"] == len(X)
    probs = model.predict_proba(X)
    assert probs.shape == (len(X), 3)
    assert np.allclose(probs.sum(axis=1), 1.0, atol=1e-5)


def test_accuracy_better_than_random():
    X, y = _synth(600, seed=1)
    model = DirectionXGB(params=XGBParams(n_estimators=80, max_depth=4))
    metrics = model.fit(X, y)
    assert metrics["train_accuracy"] > 0.34


def test_save_load_roundtrip(tmp_path):
    X, y = _synth(300, seed=2)
    model = DirectionXGB(params=XGBParams(n_estimators=40, max_depth=3))
    model.fit(X, y)
    model.save(tmp_path)
    loaded = DirectionXGB()
    loaded.load(tmp_path)
    np.testing.assert_allclose(model.predict_proba(X), loaded.predict_proba(X), atol=1e-5)
