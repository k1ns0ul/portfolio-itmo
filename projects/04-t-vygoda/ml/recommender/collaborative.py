from __future__ import annotations

import logging
from dataclasses import dataclass
from pathlib import Path

import numpy as np
import pandas as pd

log = logging.getLogger(__name__)


@dataclass
class ALSParams:
    n_factors: int = 32
    n_iterations: int = 15
    regularization: float = 0.1
    alpha: float = 40.0
    random_state: int = 42


class CollaborativeFilter:
    def __init__(self, params: ALSParams | None = None) -> None:
        self.params = params or ALSParams()
        self.user_factors: np.ndarray | None = None
        self.item_factors: np.ndarray | None = None
        self.user_index: dict[int, int] = {}
        self.item_index: dict[int, int] = {}
        self.user_inverse: dict[int, int] = {}
        self.item_inverse: dict[int, int] = {}

    @staticmethod
    def build_matrix(
        interactions: pd.DataFrame,
        user_col: str = "user_id",
        item_col: str = "category_id",
        value_col: str | None = "amount",
    ) -> tuple[np.ndarray, dict[int, int], dict[int, int]]:
        df = interactions.dropna(subset=[user_col, item_col]).copy()
        df[user_col] = df[user_col].astype("int64")
        df[item_col] = df[item_col].astype("int64")
        users = sorted(df[user_col].unique().tolist())
        items = sorted(df[item_col].unique().tolist())
        u_idx = {u: i for i, u in enumerate(users)}
        i_idx = {it: i for i, it in enumerate(items)}
        mat = np.zeros((len(users), len(items)), dtype=np.float32)
        if value_col is not None and value_col in df.columns:
            agg = df.groupby([user_col, item_col])[value_col].sum().reset_index()
        else:
            agg = df.groupby([user_col, item_col]).size().reset_index(name="count")
            value_col = "count"
        for _, row in agg.iterrows():
            mat[u_idx[int(row[user_col])], i_idx[int(row[item_col])]] = float(row[value_col])
        return mat, u_idx, i_idx

    def fit(
        self,
        interaction_matrix: np.ndarray,
        user_index: dict[int, int],
        item_index: dict[int, int],
    ) -> None:
        if interaction_matrix.size == 0:
            raise ValueError("empty interaction matrix")

        self.user_index = user_index
        self.item_index = item_index
        self.user_inverse = {v: k for k, v in user_index.items()}
        self.item_inverse = {v: k for k, v in item_index.items()}

        rng = np.random.default_rng(self.params.random_state)
        n_users, n_items = interaction_matrix.shape
        f = self.params.n_factors

        X = rng.standard_normal((n_users, f)).astype(np.float32) * 0.01
        Y = rng.standard_normal((n_items, f)).astype(np.float32) * 0.01

        confidence = 1.0 + self.params.alpha * interaction_matrix
        preference = (interaction_matrix > 0).astype(np.float32)
        reg = self.params.regularization * np.eye(f, dtype=np.float32)

        for it in range(self.params.n_iterations):
            X = self._update_side(confidence, preference, Y, reg)
            Y = self._update_side(confidence.T, preference.T, X, reg)
            train_err = self._reconstruction_error(X, Y, preference, confidence)
            log.info("als iter=%d err=%.5f", it + 1, train_err)

        self.user_factors = X
        self.item_factors = Y

    def _update_side(
        self,
        confidence: np.ndarray,
        preference: np.ndarray,
        other: np.ndarray,
        reg: np.ndarray,
    ) -> np.ndarray:
        f = other.shape[1]
        YtY = other.T @ other
        result = np.empty((confidence.shape[0], f), dtype=np.float32)
        for u in range(confidence.shape[0]):
            c_u = confidence[u]
            p_u = preference[u]
            weighted = (other.T * (c_u - 1.0)) @ other
            A = YtY + weighted + reg
            b = other.T @ (c_u * p_u)
            result[u] = np.linalg.solve(A, b)
        return result

    def _reconstruction_error(
        self,
        X: np.ndarray,
        Y: np.ndarray,
        preference: np.ndarray,
        confidence: np.ndarray,
    ) -> float:
        pred = X @ Y.T
        diff = preference - pred
        weighted = confidence * (diff ** 2)
        return float(weighted.mean())

    def recommend(self, user_id: int, n: int = 10, exclude_seen: bool = False, seen_items: set[int] | None = None) -> list[tuple[int, float]]:
        self._require_trained()
        if user_id not in self.user_index:
            return []
        u = self.user_index[user_id]
        scores = self.user_factors[u] @ self.item_factors.T
        if exclude_seen and seen_items:
            seen_indices = [self.item_index[i] for i in seen_items if i in self.item_index]
            if seen_indices:
                scores[seen_indices] = -np.inf
        top = np.argsort(-scores)[:n]
        return [(self.item_inverse[int(i)], float(scores[int(i)])) for i in top]

    def similar_users(self, user_id: int, n: int = 5) -> list[tuple[int, float]]:
        self._require_trained()
        if user_id not in self.user_index:
            return []
        u = self.user_index[user_id]
        vec = self.user_factors[u]
        norms = np.linalg.norm(self.user_factors, axis=1)
        sims = (self.user_factors @ vec) / (norms * (np.linalg.norm(vec) + 1e-9) + 1e-9)
        sims[u] = -np.inf
        top = np.argsort(-sims)[:n]
        return [(self.user_inverse[int(i)], float(sims[int(i)])) for i in top]

    def score_for(self, user_id: int, item_id: int) -> float:
        self._require_trained()
        if user_id not in self.user_index or item_id not in self.item_index:
            return 0.0
        u = self.user_index[user_id]
        i = self.item_index[item_id]
        return float(self.user_factors[u] @ self.item_factors[i])

    def save(self, path: str | Path) -> None:
        self._require_trained()
        path = Path(path)
        path.parent.mkdir(parents=True, exist_ok=True)
        np.savez(
            path,
            user_factors=self.user_factors,
            item_factors=self.item_factors,
            user_index=np.array(list(self.user_index.items()), dtype=np.int64),
            item_index=np.array(list(self.item_index.items()), dtype=np.int64),
            params=np.array([
                self.params.n_factors,
                self.params.n_iterations,
                self.params.regularization,
                self.params.alpha,
                self.params.random_state,
            ], dtype=np.float64),
        )

    def load(self, path: str | Path) -> None:
        blob = np.load(path, allow_pickle=False)
        self.user_factors = blob["user_factors"]
        self.item_factors = blob["item_factors"]
        u_idx = blob["user_index"]
        i_idx = blob["item_index"]
        self.user_index = {int(k): int(v) for k, v in u_idx}
        self.item_index = {int(k): int(v) for k, v in i_idx}
        self.user_inverse = {v: k for k, v in self.user_index.items()}
        self.item_inverse = {v: k for k, v in self.item_index.items()}
        p = blob["params"]
        self.params = ALSParams(
            n_factors=int(p[0]),
            n_iterations=int(p[1]),
            regularization=float(p[2]),
            alpha=float(p[3]),
            random_state=int(p[4]),
        )

    def _require_trained(self) -> None:
        if self.user_factors is None or self.item_factors is None:
            raise RuntimeError("ALS is not trained")
