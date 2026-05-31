from __future__ import annotations

import logging
from datetime import datetime, timezone
from typing import Any, Iterable

import asyncpg
import pandas as pd

from config import MLConfig

log = logging.getLogger(__name__)


class PostgresDB:
    def __init__(self, cfg: MLConfig) -> None:
        self.cfg = cfg
        self.pool: asyncpg.Pool | None = None

    async def connect(self) -> None:
        if self.pool is not None:
            return
        self.pool = await asyncpg.create_pool(
            dsn=self.cfg.pg_dsn,
            min_size=2,
            max_size=20,
            command_timeout=30,
        )
        log.info("postgres pool ready")

    async def close(self) -> None:
        if self.pool is not None:
            await self.pool.close()
            self.pool = None

    async def fetch_user_purchases(self, user_id: int) -> list[dict[str, Any]]:
        q = """
            SELECT p.id, p.user_id, p.promo_id, p.partner_id, p.amount,
                   p.cpa_amount, p.status, p.created_at, p.confirmed_at,
                   pr.code AS promo_code, pr.category_id, pr.discount
            FROM purchases p
            JOIN promos pr ON pr.id = p.promo_id
            WHERE p.user_id = $1
            ORDER BY p.created_at DESC
        """
        rows = await self.pool.fetch(q, user_id)
        return [dict(r) for r in rows]

    async def fetch_all_purchases(self, since_days: int = 90) -> pd.DataFrame:
        q = """
            SELECT p.id, p.user_id, p.promo_id, p.partner_id, p.amount,
                   p.cpa_amount, p.status, p.created_at,
                   pr.category_id
            FROM purchases p
            JOIN promos pr ON pr.id = p.promo_id
            WHERE p.created_at >= now() - INTERVAL '1 day' * $1
        """
        rows = await self.pool.fetch(q, since_days)
        return pd.DataFrame([dict(r) for r in rows])

    async def fetch_user_categories(self, user_id: int) -> list[int]:
        q = """
            SELECT DISTINCT pr.category_id
            FROM purchases p
            JOIN promos pr ON pr.id = p.promo_id
            WHERE p.user_id = $1 AND pr.category_id IS NOT NULL
        """
        rows = await self.pool.fetch(q, user_id)
        return [r["category_id"] for r in rows]

    async def fetch_promos_active(self) -> pd.DataFrame:
        q = """
            SELECT pr.id, pr.partner_id, pr.code, pr.discount, pr.type, pr.category_id,
                   pr.max_uses, pr.current_uses, pr.expires_at, pr.created_at,
                   pa.cpa_percent
            FROM promos pr
            JOIN partners pa ON pa.id = pr.partner_id
            WHERE pr.active = TRUE
              AND (pr.expires_at IS NULL OR pr.expires_at > now())
              AND (pr.max_uses = 0 OR pr.current_uses < pr.max_uses)
              AND pa.active = TRUE
        """
        rows = await self.pool.fetch(q)
        return pd.DataFrame([dict(r) for r in rows])

    async def fetch_referral_edges(self) -> list[tuple[int, int]]:
        q = """
            SELECT referrer_id, user_id FROM referral_chains WHERE level = 1
        """
        rows = await self.pool.fetch(q)
        return [(r["referrer_id"], r["user_id"]) for r in rows]

    async def fetch_referral_bonuses(self, since_days: int = 30) -> pd.DataFrame:
        q = """
            SELECT id, referrer_id, purchase_id, amount, level, status, created_at
            FROM referral_bonuses
            WHERE created_at >= now() - INTERVAL '1 day' * $1
        """
        rows = await self.pool.fetch(q, since_days)
        return pd.DataFrame([dict(r) for r in rows])

    async def fetch_user_profile(self, user_id: int) -> dict[str, Any] | None:
        q = """
            SELECT id, phone, name, referred_by, level, created_at,
                   (SELECT COUNT(*) FROM referral_chains WHERE referrer_id = u.id AND level = 1) AS direct_referrals
            FROM users u WHERE id = $1
        """
        row = await self.pool.fetchrow(q, user_id)
        return dict(row) if row else None

    async def list_active_user_ids(self, days: int = 30) -> list[int]:
        q = """
            SELECT DISTINCT user_id
            FROM purchases WHERE created_at >= now() - INTERVAL '1 day' * $1
        """
        rows = await self.pool.fetch(q, days)
        return [r["user_id"] for r in rows]

    async def write_recommendations(self, user_id: int, recs: Iterable[dict[str, Any]]) -> None:
        now = datetime.now(timezone.utc)
        rows: list[tuple[int, int, float, str | None, datetime]] = []
        for r in recs:
            rows.append((
                user_id,
                int(r["promo_id"]),
                float(r["score"]),
                r.get("reason"),
                now,
            ))
        if not rows:
            return
        async with self.pool.acquire() as conn:
            async with conn.transaction():
                await conn.execute(
                    "DELETE FROM recommendations WHERE user_id = $1", user_id
                )
                await conn.executemany(
                    """
                    INSERT INTO recommendations (user_id, promo_id, score, reason, generated_at)
                    VALUES ($1, $2, $3, $4, $5)
                    ON CONFLICT (user_id, promo_id) DO UPDATE
                    SET score = EXCLUDED.score,
                        reason = EXCLUDED.reason,
                        generated_at = EXCLUDED.generated_at
                    """,
                    rows,
                )
