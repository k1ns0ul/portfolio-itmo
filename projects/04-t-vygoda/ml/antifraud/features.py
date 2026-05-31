from __future__ import annotations

import logging
from datetime import datetime

import networkx as nx
import numpy as np
import pandas as pd

log = logging.getLogger(__name__)


FRAUD_FEATURE_COLUMNS: list[str] = [
    "degree",
    "depth",
    "subtree_size",
    "subtree_bonuses_sum",
    "avg_time_between_referrals",
    "conversion_rate",
    "same_ip_ratio",
]


class FraudFeatures:
    def __init__(
        self,
        graph: nx.DiGraph,
        bonuses: pd.DataFrame | None = None,
        edge_timestamps: dict[tuple[int, int], datetime] | None = None,
        purchases_by_user: dict[int, int] | None = None,
        ip_groups: dict[int, str] | None = None,
    ) -> None:
        self.graph = graph
        self.bonuses = bonuses if bonuses is not None else pd.DataFrame()
        self.edge_timestamps = edge_timestamps or {}
        self.purchases_by_user = purchases_by_user or {}
        self.ip_groups = ip_groups or {}

    def build(self) -> pd.DataFrame:
        nodes = list(self.graph.nodes())
        if not nodes:
            return pd.DataFrame(columns=["user_id"] + FRAUD_FEATURE_COLUMNS).set_index("user_id")

        depth = self._compute_depth()
        subtree_sizes = self._compute_subtree_sizes()
        bonus_sums = self._compute_bonus_subtree_sums(subtree_sizes)
        time_gaps = self._compute_time_gaps()
        records: list[dict] = []
        for node in nodes:
            children = list(self.graph.successors(node))
            conv = self._conversion_rate(children)
            records.append({
                "user_id": int(node),
                "degree": int(len(children)),
                "depth": float(depth.get(node, 0.0)),
                "subtree_size": int(subtree_sizes.get(node, 1)),
                "subtree_bonuses_sum": float(bonus_sums.get(node, 0.0)),
                "avg_time_between_referrals": float(time_gaps.get(node, 0.0)),
                "conversion_rate": float(conv),
                "same_ip_ratio": float(self._same_ip_ratio(node, children)),
            })
        out = pd.DataFrame(records).set_index("user_id")
        out = out[FRAUD_FEATURE_COLUMNS].astype(float)
        log.info("fraud features built for %d users", len(out))
        return out

    def _compute_depth(self) -> dict[int, int]:
        depth: dict[int, int] = {}
        roots = [n for n, d in self.graph.in_degree() if d == 0]
        for root in roots:
            for node, dist in nx.single_source_shortest_path_length(self.graph, root).items():
                if node not in depth or dist < depth[node]:
                    depth[node] = dist
        return depth

    def _compute_subtree_sizes(self) -> dict[int, int]:
        sizes: dict[int, int] = {}
        for node in self.graph.nodes():
            descendants = nx.descendants(self.graph, node)
            sizes[node] = 1 + len(descendants)
        return sizes

    def _compute_bonus_subtree_sums(self, subtree_sizes: dict[int, int]) -> dict[int, float]:
        if self.bonuses.empty:
            return {n: 0.0 for n in self.graph.nodes()}
        per_user = self.bonuses.groupby("referrer_id")["amount"].sum().to_dict()
        out: dict[int, float] = {}
        for node in self.graph.nodes():
            descendants = nx.descendants(self.graph, node)
            total = float(per_user.get(node, 0.0))
            for d in descendants:
                total += float(per_user.get(d, 0.0))
            out[node] = total
        return out

    def _compute_time_gaps(self) -> dict[int, float]:
        if not self.edge_timestamps:
            return {}
        by_referrer: dict[int, list[float]] = {}
        for (referrer, _user), ts in self.edge_timestamps.items():
            by_referrer.setdefault(referrer, []).append(ts.timestamp())
        out: dict[int, float] = {}
        for referrer, stamps in by_referrer.items():
            if len(stamps) < 2:
                out[referrer] = 0.0
                continue
            stamps.sort()
            diffs = np.diff(stamps)
            out[referrer] = float(np.mean(diffs))
        return out

    def _conversion_rate(self, children: list[int]) -> float:
        if not children:
            return 0.0
        purchased = sum(1 for c in children if self.purchases_by_user.get(int(c), 0) > 0)
        return purchased / len(children)

    def _same_ip_ratio(self, node: int, children: list[int]) -> float:
        if not children or not self.ip_groups:
            return 0.0
        own_ip = self.ip_groups.get(node)
        if not own_ip:
            return 0.0
        same = sum(1 for c in children if self.ip_groups.get(int(c)) == own_ip)
        return same / len(children)
