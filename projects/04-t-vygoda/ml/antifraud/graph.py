from __future__ import annotations

import logging
from collections import defaultdict
from dataclasses import dataclass, field
from datetime import datetime, timezone

import networkx as nx
import numpy as np
import pandas as pd

log = logging.getLogger(__name__)


@dataclass
class AuditReport:
    total_users: int
    rings: list[list[int]] = field(default_factory=list)
    dense_clusters: list[list[int]] = field(default_factory=list)
    velocity_abusers: list[int] = field(default_factory=list)
    bonus_outliers: list[int] = field(default_factory=list)


class ReferralGraph:
    def __init__(
        self,
        edges: list[tuple[int, int]],
        edge_timestamps: dict[tuple[int, int], datetime] | None = None,
        bonuses: pd.DataFrame | None = None,
    ) -> None:
        self.graph: nx.DiGraph = nx.DiGraph()
        self.graph.add_edges_from(edges)
        self.edge_timestamps = edge_timestamps or {}
        self.bonuses = bonuses if bonuses is not None else pd.DataFrame()

    def detect_rings(self, max_length: int = 5) -> list[list[int]]:
        if self.graph.number_of_nodes() == 0:
            return []
        rings: list[list[int]] = []
        seen: set[tuple[int, ...]] = set()
        for cycle in nx.simple_cycles(self.graph):
            if len(cycle) < 2 or len(cycle) > max_length:
                continue
            normalized = self._normalize_cycle(cycle)
            if normalized in seen:
                continue
            seen.add(normalized)
            rings.append(cycle)
        log.info("detected %d rings (max_length=%d)", len(rings), max_length)
        return rings

    def detect_clusters(self, min_size: int = 5, density_threshold: float = 0.6) -> list[set[int]]:
        if self.graph.number_of_nodes() == 0:
            return []
        undirected = self.graph.to_undirected()
        communities = self._louvain_like(undirected)
        suspicious: list[set[int]] = []
        for comm in communities:
            if len(comm) < min_size:
                continue
            density = self._density(undirected.subgraph(comm))
            if density >= density_threshold:
                suspicious.append(set(comm))
        log.info("detected %d dense clusters", len(suspicious))
        return suspicious

    def detect_velocity_anomalies(self, window_hours: int = 24, threshold: int = 10) -> list[int]:
        if not self.edge_timestamps:
            return self._velocity_without_time(threshold)
        cutoff = datetime.now(timezone.utc) - pd.Timedelta(hours=window_hours).to_pytimedelta()
        recent: dict[int, int] = defaultdict(int)
        for (referrer, _user), ts in self.edge_timestamps.items():
            if ts >= cutoff:
                recent[referrer] += 1
        abusers = [uid for uid, count in recent.items() if count > threshold]
        log.info("velocity abusers: %d", len(abusers))
        return abusers

    def detect_bonus_anomalies(self, top_percentile: float = 0.99) -> list[int]:
        if self.bonuses.empty:
            return []
        totals = self.bonuses.groupby("referrer_id")["amount"].sum()
        if totals.empty:
            return []
        threshold = float(np.quantile(totals.to_numpy(), top_percentile))
        outliers = totals[totals > threshold].index.astype(int).tolist()
        log.info("bonus outliers: %d (threshold=%.2f)", len(outliers), threshold)
        return outliers

    def full_scan(
        self,
        ring_max_length: int = 5,
        velocity_threshold: int = 10,
        window_hours: int = 24,
        bonus_percentile: float = 0.99,
        cluster_min_size: int = 5,
        cluster_density: float = 0.6,
    ) -> AuditReport:
        return AuditReport(
            total_users=self.graph.number_of_nodes(),
            rings=self.detect_rings(max_length=ring_max_length),
            dense_clusters=[list(c) for c in self.detect_clusters(cluster_min_size, cluster_density)],
            velocity_abusers=self.detect_velocity_anomalies(window_hours, velocity_threshold),
            bonus_outliers=self.detect_bonus_anomalies(bonus_percentile),
        )

    def _normalize_cycle(self, cycle: list[int]) -> tuple[int, ...]:
        if not cycle:
            return ()
        idx = cycle.index(min(cycle))
        rotated = cycle[idx:] + cycle[:idx]
        return tuple(rotated)

    def _density(self, sub: nx.Graph) -> float:
        n = sub.number_of_nodes()
        if n < 2:
            return 0.0
        max_edges = n * (n - 1) / 2.0
        return float(sub.number_of_edges()) / max_edges

    def _louvain_like(self, undirected: nx.Graph) -> list[list[int]]:
        try:
            from networkx.algorithms.community import louvain_communities

            comms = louvain_communities(undirected, seed=42)
            return [list(c) for c in comms]
        except (ImportError, AttributeError) as e:
            log.warning("louvain unavailable, falling back to connected_components: %s", e)
            return [list(c) for c in nx.connected_components(undirected)]

    def _velocity_without_time(self, threshold: int) -> list[int]:
        return [n for n, deg in self.graph.out_degree() if deg > threshold]
