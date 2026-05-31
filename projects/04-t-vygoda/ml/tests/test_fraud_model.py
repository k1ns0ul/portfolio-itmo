from __future__ import annotations

import numpy as np
import pandas as pd

from antifraud.features import FRAUD_FEATURE_COLUMNS
from antifraud.model import DetectorParams, FraudDetector


def _synth_features(n: int = 200, n_anomalies: int = 10, seed: int = 0) -> pd.DataFrame:
    rng = np.random.default_rng(seed)
    rows = []
    for i in range(n - n_anomalies):
        rows.append({
            "user_id": i,
            "degree": int(rng.integers(0, 5)),
            "depth": float(rng.integers(0, 4)),
            "subtree_size": int(rng.integers(1, 10)),
            "subtree_bonuses_sum": float(rng.uniform(0, 100)),
            "avg_time_between_referrals": float(rng.uniform(1e5, 1e6)),
            "conversion_rate": float(rng.uniform(0.2, 0.9)),
            "same_ip_ratio": float(rng.uniform(0.0, 0.2)),
        })
    for i in range(n_anomalies):
        rows.append({
            "user_id": 10_000 + i,
            "degree": int(rng.integers(40, 80)),
            "depth": float(rng.integers(0, 2)),
            "subtree_size": int(rng.integers(300, 600)),
            "subtree_bonuses_sum": float(rng.uniform(50_000, 100_000)),
            "avg_time_between_referrals": float(rng.uniform(5, 60)),
            "conversion_rate": float(rng.uniform(0.0, 0.05)),
            "same_ip_ratio": float(rng.uniform(0.7, 0.95)),
        })
    df = pd.DataFrame(rows).set_index("user_id")
    return df[FRAUD_FEATURE_COLUMNS]


def test_unsupervised_finds_anomalies():
    features = _synth_features()
    detector = FraudDetector(params=DetectorParams(contamination=0.05))
    detector.fit(features)
    preds = detector.predict(features)
    flagged_ids = {p.user_id for p in preds if p.is_fraud}
    anomaly_ids = set(features.index[-10:])
    overlap = flagged_ids & anomaly_ids
    assert len(overlap) >= 5


def test_save_load_roundtrip(tmp_path):
    features = _synth_features()
    detector = FraudDetector(params=DetectorParams(contamination=0.05))
    detector.fit(features)
    detector.save(tmp_path)

    loaded = FraudDetector()
    loaded.load(tmp_path)
    p1 = sorted(detector.predict(features), key=lambda p: p.user_id)
    p2 = sorted(loaded.predict(features), key=lambda p: p.user_id)
    flags1 = [p.is_fraud for p in p1]
    flags2 = [p.is_fraud for p in p2]
    assert flags1 == flags2
