from __future__ import annotations

import numpy as np
import pandas as pd

from recommender.collaborative import ALSParams, CollaborativeFilter


def _synth_matrix() -> tuple[np.ndarray, dict[int, int], dict[int, int]]:
    rng = np.random.default_rng(0)
    n_users, n_items = 20, 8
    mat = np.zeros((n_users, n_items), dtype=np.float32)
    for u in range(n_users):
        prefer_block = u % 2
        for i in range(n_items):
            if i % 2 == prefer_block and rng.random() < 0.7:
                mat[u, i] = rng.integers(1, 5)
    u_idx = {u: u for u in range(n_users)}
    i_idx = {i: i for i in range(n_items)}
    return mat, u_idx, i_idx


def test_fit_produces_factors():
    mat, u_idx, i_idx = _synth_matrix()
    cf = CollaborativeFilter(ALSParams(n_factors=4, n_iterations=5, regularization=0.1, alpha=10.0))
    cf.fit(mat, u_idx, i_idx)
    assert cf.user_factors.shape == (20, 4)
    assert cf.item_factors.shape == (8, 4)


def test_recommend_returns_requested_count():
    mat, u_idx, i_idx = _synth_matrix()
    cf = CollaborativeFilter(ALSParams(n_factors=4, n_iterations=5))
    cf.fit(mat, u_idx, i_idx)
    recs = cf.recommend(0, n=3)
    assert len(recs) == 3
    item_ids = {it for it, _ in recs}
    assert all(0 <= it < 8 for it in item_ids)


def test_recommend_unknown_user():
    mat, u_idx, i_idx = _synth_matrix()
    cf = CollaborativeFilter(ALSParams(n_factors=4, n_iterations=5))
    cf.fit(mat, u_idx, i_idx)
    assert cf.recommend(999, n=5) == []


def test_similar_users_returns_other_users():
    mat, u_idx, i_idx = _synth_matrix()
    cf = CollaborativeFilter(ALSParams(n_factors=4, n_iterations=5))
    cf.fit(mat, u_idx, i_idx)
    sims = cf.similar_users(0, n=3)
    assert len(sims) == 3
    assert all(uid != 0 for uid, _ in sims)


def test_save_load_roundtrip(tmp_path):
    mat, u_idx, i_idx = _synth_matrix()
    cf = CollaborativeFilter(ALSParams(n_factors=4, n_iterations=3))
    cf.fit(mat, u_idx, i_idx)
    path = tmp_path / "collab.npz"
    cf.save(path)

    loaded = CollaborativeFilter()
    loaded.load(path)
    s1 = cf.recommend(0, n=5)
    s2 = loaded.recommend(0, n=5)
    assert [it for it, _ in s1] == [it for it, _ in s2]


def test_build_matrix_from_dataframe():
    df = pd.DataFrame([
        {"user_id": 1, "category_id": 10, "amount": 5.0},
        {"user_id": 1, "category_id": 20, "amount": 2.0},
        {"user_id": 2, "category_id": 10, "amount": 1.0},
    ])
    mat, u_idx, i_idx = CollaborativeFilter.build_matrix(df, value_col="amount")
    assert mat.shape == (2, 2)
    assert mat[u_idx[1], i_idx[10]] == 5.0
