from __future__ import annotations

from datetime import datetime, timedelta, timezone

import numpy as np
import pandas as pd

from features import FEATURE_COLUMNS, FeatureExtractor, HeuristicLabeler


def _make_raw(rng: np.random.Generator, n_wallets: int = 20, per_wallet: int = 30) -> pd.DataFrame:
    base = datetime(2025, 1, 1, tzinfo=timezone.utc)
    rows = []
    wallets = [f"W{i:03d}" for i in range(n_wallets)]
    programs = ["11111111111111111111111111111111",
                "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA",
                "675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8"]
    for i, w in enumerate(wallets):
        for j in range(per_wallet):
            counter = wallets[(i + 1 + (j % 3)) % n_wallets]
            rows.append({
                "signature": f"sig-{i}-{j}",
                "slot": 1000 + j,
                "block_time": base + timedelta(hours=i * 2 + j),
                "fee": 5000,
                "sender": w,
                "receiver": counter,
                "amount": int(rng.integers(1, 1_000_000_000)),
                "program_id": programs[j % len(programs)],
                "swap_kind": "",
                "success": True,
            })
    return pd.DataFrame(rows)


def test_extractor_returns_expected_columns():
    rng = np.random.default_rng(0)
    raw = _make_raw(rng, n_wallets=5, per_wallet=10)
    extractor = FeatureExtractor()
    features = extractor.extract(raw)
    assert not features.empty
    assert list(features.columns) == FEATURE_COLUMNS
    assert (features["tx_count"] >= 1).all()


def test_extractor_empty_input():
    features = FeatureExtractor().extract(pd.DataFrame())
    assert features.empty
    assert list(features.columns) == FEATURE_COLUMNS


def test_extractor_handles_single_transaction_wallet():
    raw = pd.DataFrame([{
        "signature": "s1", "slot": 1,
        "block_time": pd.Timestamp("2025-01-01", tz="UTC"),
        "fee": 5000, "sender": "A", "receiver": "B", "amount": 1_000_000,
        "program_id": "11111111111111111111111111111111", "swap_kind": "", "success": True,
    }])
    features = FeatureExtractor().extract(raw)
    assert features.loc["A", "tx_count"] == 1
    assert features.loc["A", "avg_time_between_tx"] == 0.0


def test_labeler_produces_three_classes():
    rng = np.random.default_rng(42)
    raw = _make_raw(rng, n_wallets=12, per_wallet=20)
    features = FeatureExtractor().extract(raw)
    labels = HeuristicLabeler().label(features, raw)
    assert set(labels["target_label"].unique()).issubset({0, 1, 2})
    assert labels["target_score"].between(0, 100).all()
