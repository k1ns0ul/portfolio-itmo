from __future__ import annotations

from datetime import datetime, timedelta, timezone

import pandas as pd

from antifraud.graph import ReferralGraph


def test_detect_rings_finds_triangle():
    edges = [(1, 2), (2, 3), (3, 1), (4, 5)]
    graph = ReferralGraph(edges)
    rings = graph.detect_rings(max_length=5)
    assert any(set(r) == {1, 2, 3} for r in rings)


def test_detect_rings_respects_max_length():
    edges = [(1, 2), (2, 3), (3, 4), (4, 5), (5, 1)]
    graph = ReferralGraph(edges)
    short_rings = graph.detect_rings(max_length=3)
    assert all(len(r) <= 3 for r in short_rings)


def test_velocity_anomalies_without_timestamps():
    edges = [(1, i) for i in range(100, 115)]
    graph = ReferralGraph(edges)
    abusers = graph.detect_velocity_anomalies(window_hours=24, threshold=10)
    assert 1 in abusers


def test_velocity_anomalies_with_timestamps():
    now = datetime.now(timezone.utc)
    edges = [(1, i) for i in range(20, 35)]
    ts = {edge: now - timedelta(hours=1) for edge in edges}
    graph = ReferralGraph(edges, edge_timestamps=ts)
    abusers = graph.detect_velocity_anomalies(window_hours=24, threshold=5)
    assert 1 in abusers


def test_bonus_outliers():
    bonuses = pd.DataFrame([
        {"referrer_id": 1, "amount": 10.0},
        {"referrer_id": 2, "amount": 15.0},
        {"referrer_id": 3, "amount": 25.0},
        {"referrer_id": 4, "amount": 5000.0},
    ])
    graph = ReferralGraph([(1, 2)], bonuses=bonuses)
    outliers = graph.detect_bonus_anomalies(top_percentile=0.9)
    assert 4 in outliers


def test_full_scan_aggregates():
    edges = [(1, 2), (2, 3), (3, 1)]
    bonuses = pd.DataFrame([{"referrer_id": 1, "amount": 100.0}])
    graph = ReferralGraph(edges, bonuses=bonuses)
    report = graph.full_scan(ring_max_length=3, velocity_threshold=2)
    assert report.total_users == 3
    assert report.rings


def test_dense_clusters_detection():
    edges = []
    members = [10, 11, 12, 13, 14, 15]
    for a in members:
        for b in members:
            if a != b:
                edges.append((a, b))
    graph = ReferralGraph(edges)
    clusters = graph.detect_clusters(min_size=5, density_threshold=0.5)
    assert clusters
