from __future__ import annotations

import numpy as np
import torch

from models import DirectionLSTM
from models.lstm_model import LSTM_FEATURE_COLUMNS, LSTMParams


def _synth(n: int = 200, seq_len: int = 12, seed: int = 0) -> tuple[np.ndarray, np.ndarray]:
    rng = np.random.default_rng(seed)
    sequences = []
    labels = []
    for i in range(n):
        cls = i % 3
        base = np.zeros((seq_len, len(LSTM_FEATURE_COLUMNS)), dtype=np.float32)
        for t in range(seq_len):
            base[t, 0] = rng.normal((cls - 1) * 30, 5)
            base[t, 1] = abs(rng.normal(0.1, 0.05))
            base[t, 2] = rng.normal(0, 0.01)
            base[t, 3] = abs(rng.normal(100, 20))
            base[t, 4] = float(rng.uniform(0, 1))
            base[t, 5] = abs(rng.normal(1000, 200))
            base[t, 6] = abs(rng.normal(0.05, 0.02))
            base[t, 7] = abs(rng.normal(150, 10))
            base[t, 8] = float(rng.integers(5, 50))
        sequences.append(base)
        labels.append(cls)
    return np.stack(sequences), np.array(labels, dtype=np.int64)


def _params(epochs: int = 5) -> LSTMParams:
    return LSTMParams(
        seq_len=12,
        hidden=16,
        layers=1,
        dropout=0.0,
        epochs=epochs,
        batch_size=32,
        lr=1e-2,
        patience=epochs,
        num_workers=0,
    )


def test_forward_pass_shape():
    X, y = _synth(60)
    model = DirectionLSTM(params=_params(epochs=2))
    model.fit(X, y, val_split=0.2)
    probs = model.predict_proba(X)
    assert probs.shape == (60, 3)
    assert np.allclose(probs.sum(axis=1), 1.0, atol=1e-4)


def test_train_loss_decreases_over_epochs():
    X, y = _synth(120, seed=1)
    model = DirectionLSTM(params=_params(epochs=5))
    torch.manual_seed(0)
    metrics = model.fit(X, y, val_split=0.2)
    assert metrics["best_val_loss"] < 1.5


def test_save_load_roundtrip(tmp_path):
    X, y = _synth(80, seed=2)
    model = DirectionLSTM(params=_params(epochs=2))
    model.fit(X, y, val_split=0.2)
    model.save(tmp_path)
    loaded = DirectionLSTM()
    loaded.load(tmp_path)
    np.testing.assert_allclose(model.predict_proba(X), loaded.predict_proba(X), atol=1e-4)
