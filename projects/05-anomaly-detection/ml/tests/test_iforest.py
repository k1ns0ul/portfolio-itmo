from __future__ import annotations

import numpy as np

from models import IForestModel
from models.iforest import IForestParams


def _synth(seed: int = 0) -> tuple[np.ndarray, np.ndarray]:
    rng = np.random.default_rng(seed)
    normal = rng.normal(loc=0.0, scale=1.0, size=(500, 6))
    anomalies = rng.normal(loc=8.0, scale=1.0, size=(20, 6))
    return normal, anomalies


def test_fit_and_predict_shapes():
    normal, _ = _synth()
    model = IForestModel(IForestParams(n_estimators=50, contamination=0.05))
    metrics = model.fit(normal)
    assert metrics["rows"] == 500
    flags = model.predict(normal)
    assert flags.shape == (500,)
    assert set(np.unique(flags)).issubset({-1, 1})


def test_scores_are_higher_on_anomalies():
    normal, anomalies = _synth()
    model = IForestModel(IForestParams(n_estimators=80, contamination=0.05))
    model.fit(normal)
    s_normal = model.scores(normal).mean()
    s_anom = model.scores(anomalies).mean()
    assert s_anom > s_normal


def test_save_load_roundtrip(tmp_path):
    normal, _ = _synth()
    model = IForestModel(IForestParams(n_estimators=50, contamination=0.05))
    model.fit(normal)
    model.save(tmp_path)
    loaded = IForestModel()
    loaded.load(tmp_path)
    np.testing.assert_array_equal(model.predict(normal), loaded.predict(normal))
