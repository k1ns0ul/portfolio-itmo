from __future__ import annotations

import numpy as np

from models import AutoencoderModel
from models.autoencoder import AutoencoderParams


def _normal(n: int = 600, dim: int = 8, seed: int = 0) -> np.ndarray:
    rng = np.random.default_rng(seed)
    base = rng.normal(loc=0.0, scale=1.0, size=(n, dim))
    base[:, 1] = 0.3 * base[:, 0] + 0.7 * rng.normal(size=n)
    return base.astype(np.float32)


def _anomalies(n: int = 30, dim: int = 8, seed: int = 1) -> np.ndarray:
    rng = np.random.default_rng(seed)
    out = rng.normal(loc=6.0, scale=2.0, size=(n, dim)).astype(np.float32)
    return out


def _params() -> AutoencoderParams:
    return AutoencoderParams(
        input_dim=8,
        hidden_dims=(16, 8, 4),
        epochs=8,
        batch_size=64,
        lr=1e-2,
        patience=4,
        val_split=0.1,
        threshold_percentile=95.0,
    )


def test_fit_returns_threshold():
    X = _normal()
    model = AutoencoderModel(_params())
    metrics = model.fit(X)
    assert metrics["threshold"] >= 0.0
    assert metrics["mean_error"] >= 0.0


def test_anomalies_have_higher_errors():
    normal = _normal()
    model = AutoencoderModel(_params())
    model.fit(normal)
    err_normal = model.predict(normal).mean()
    err_anom = model.predict(_anomalies()).mean()
    assert err_anom > err_normal


def test_is_anomaly_flags_injected():
    normal = _normal()
    model = AutoencoderModel(_params())
    model.fit(normal)
    flags = model.is_anomaly(_anomalies())
    assert flags.mean() > 0.5


def test_save_load_roundtrip(tmp_path):
    normal = _normal()
    model = AutoencoderModel(_params())
    model.fit(normal)
    model.save(tmp_path)
    loaded = AutoencoderModel()
    loaded.load(tmp_path)
    np.testing.assert_allclose(model.predict(normal), loaded.predict(normal), atol=1e-5)
