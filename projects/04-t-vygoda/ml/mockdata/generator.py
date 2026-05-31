from __future__ import annotations

import asyncio
import logging
import random
import secrets
import string
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone

import asyncpg

from config import MLConfig

log = logging.getLogger(__name__)


@dataclass
class GenSpec:
    n_users: int = 5000
    n_categories: int = 20
    n_partners: int = 50
    n_promos: int = 200
    n_purchases: int = 30000
    referral_share: float = 0.7
    max_depth: int = 5
    ring_count: int = 3
    cluster_count: int = 2
    cluster_size: int = 8
    velocity_abuser_count: int = 5


CATEGORY_NAMES = [
    "Электроника", "Одежда", "Книги", "Спорт", "Дом",
    "Красота", "Путешествия", "Еда", "Авто", "Игры",
    "Дети", "Подарки", "Аптека", "Музыка", "Хобби",
    "Финансы", "Образование", "Сад", "Зоотовары", "Услуги",
]


class MockGenerator:
    def __init__(self, cfg: MLConfig, spec: GenSpec | None = None) -> None:
        self.cfg = cfg
        self.spec = spec or GenSpec()
        self.rng = random.Random(cfg.random_state)

    async def run(self) -> dict:
        conn = await asyncpg.connect(dsn=self.cfg.pg_dsn)
        try:
            await self._reset(conn)
            cat_ids = await self._insert_categories(conn)
            partner_ids = await self._insert_partners(conn)
            promo_ids = await self._insert_promos(conn, partner_ids, cat_ids)
            user_ids = await self._insert_users(conn)
            await self._build_referrals(conn, user_ids)
            await self._inject_fraud_patterns(conn, user_ids)
            await self._insert_purchases(conn, user_ids, promo_ids, partner_ids)

            stats = {
                "users": len(user_ids),
                "categories": len(cat_ids),
                "partners": len(partner_ids),
                "promos": len(promo_ids),
                "purchases": self.spec.n_purchases,
            }
            log.info("mock data ready: %s", stats)
            return stats
        finally:
            await conn.close()

    async def _reset(self, conn: asyncpg.Connection) -> None:
        await conn.execute("""
            TRUNCATE TABLE recommendations, post_likes, posts, referral_bonuses,
                           referral_chains, cfa_reconciliations, cfa_settlements,
                           cfa_balances, purchases, promos, partners, categories, users
                           RESTART IDENTITY CASCADE
        """)

    async def _insert_categories(self, conn: asyncpg.Connection) -> list[int]:
        rows = []
        for name in CATEGORY_NAMES[:self.spec.n_categories]:
            slug = _slug(name)
            rows.append((name, slug, None))
        await conn.executemany(
            "INSERT INTO categories (name, slug, parent_id) VALUES ($1, $2, $3)",
            rows,
        )
        return [r["id"] for r in await conn.fetch("SELECT id FROM categories ORDER BY id")]

    async def _insert_partners(self, conn: asyncpg.Connection) -> list[int]:
        rows = []
        for i in range(self.spec.n_partners):
            rows.append((
                f"Partner #{i+1}",
                None,
                round(self.rng.uniform(0.5, 7.5), 2),
                f"partner{i+1}@example.com",
                True,
            ))
        await conn.executemany(
            """
            INSERT INTO partners (name, logo_url, cpa_percent, contact_email, active)
            VALUES ($1, $2, $3, $4, $5)
            """,
            rows,
        )
        return [r["id"] for r in await conn.fetch("SELECT id FROM partners ORDER BY id")]

    async def _insert_promos(self, conn: asyncpg.Connection, partner_ids: list[int], cat_ids: list[int]) -> list[int]:
        rows = []
        used_codes: set[str] = set()
        for _ in range(self.spec.n_promos):
            code = _new_code(used_codes)
            partner_id = self.rng.choice(partner_ids)
            cat_id = self.rng.choice(cat_ids)
            ptype = self.rng.choice(["percent", "fixed"])
            discount = self.rng.uniform(5, 70) if ptype == "percent" else self.rng.uniform(100, 5000)
            max_uses = self.rng.choice([0, 100, 500, 1000])
            expires = datetime.now(timezone.utc) + timedelta(days=self.rng.randint(7, 180))
            rows.append((partner_id, code, round(discount, 2), ptype, cat_id, max_uses, expires))
        await conn.executemany(
            """
            INSERT INTO promos (partner_id, code, discount, type, category_id, max_uses, expires_at)
            VALUES ($1, $2, $3, $4, $5, $6, $7)
            """,
            rows,
        )
        return [r["id"] for r in await conn.fetch("SELECT id FROM promos ORDER BY id")]

    async def _insert_users(self, conn: asyncpg.Connection) -> list[int]:
        rows = []
        used_phones: set[str] = set()
        used_codes: set[str] = set()
        for i in range(self.spec.n_users):
            phone = _new_phone(used_phones, self.rng)
            ref_code = _new_code(used_codes)
            rows.append((phone, f"User-{i+1}", None, ref_code))
        await conn.executemany(
            "INSERT INTO users (phone, name, email, referral_code) VALUES ($1, $2, $3, $4)",
            rows,
        )
        return [r["id"] for r in await conn.fetch("SELECT id FROM users ORDER BY id")]

    async def _build_referrals(self, conn: asyncpg.Connection, user_ids: list[int]) -> None:
        n = len(user_ids)
        target_count = int(n * self.spec.referral_share)
        if target_count < 2:
            return
        referred = set(self.rng.sample(user_ids[1:], min(target_count, n - 1)))
        chain_rows: list[tuple[int, int, int]] = []
        for user_id in referred:
            depth = self.rng.randint(1, self.spec.max_depth)
            ancestors = self._pick_ancestor_chain(user_ids, user_id, depth)
            for level, ancestor in enumerate(ancestors, start=1):
                if level > 3:
                    break
                chain_rows.append((user_id, ancestor, level))
            if ancestors:
                await conn.execute(
                    "UPDATE users SET referred_by = $1 WHERE id = $2",
                    ancestors[0], user_id,
                )
        if chain_rows:
            await conn.executemany(
                "INSERT INTO referral_chains (user_id, referrer_id, level) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING",
                chain_rows,
            )

    def _pick_ancestor_chain(self, user_ids: list[int], user_id: int, depth: int) -> list[int]:
        candidates = [u for u in user_ids if u < user_id]
        if not candidates:
            return []
        chain: list[int] = []
        current = self.rng.choice(candidates)
        chain.append(current)
        for _ in range(depth - 1):
            higher = [u for u in user_ids if u < current]
            if not higher:
                break
            current = self.rng.choice(higher)
            chain.append(current)
        return chain

    async def _inject_fraud_patterns(self, conn: asyncpg.Connection, user_ids: list[int]) -> None:
        rows_chains: list[tuple[int, int, int]] = []
        rows_bonuses: list[tuple[int, int, int, float, int, str]] = []

        for _ in range(self.spec.ring_count):
            members = self.rng.sample(user_ids, self.rng.randint(4, 6))
            for i, src in enumerate(members):
                dst = members[(i + 1) % len(members)]
                rows_chains.append((dst, src, 1))

        for _ in range(self.spec.cluster_count):
            members = self.rng.sample(user_ids, self.spec.cluster_size)
            root = members[0]
            for child in members[1:]:
                rows_chains.append((child, root, 1))

        abusers = self.rng.sample(user_ids, self.spec.velocity_abuser_count)
        non_abusers = [u for u in user_ids if u not in abusers]
        for abuser in abusers:
            invited = self.rng.sample(non_abusers, 20)
            for child in invited:
                rows_chains.append((child, abuser, 1))

        await conn.executemany(
            """
            INSERT INTO referral_chains (user_id, referrer_id, level)
            VALUES ($1, $2, $3)
            ON CONFLICT DO NOTHING
            """,
            rows_chains,
        )

    async def _insert_purchases(
        self,
        conn: asyncpg.Connection,
        user_ids: list[int],
        promo_ids: list[int],
        partner_ids: list[int],
    ) -> None:
        heavy_buyers = self.rng.sample(user_ids, int(len(user_ids) * 0.2))
        promo_to_partner: dict[int, int] = {}
        for r in await conn.fetch("SELECT id, partner_id FROM promos"):
            promo_to_partner[int(r["id"])] = int(r["partner_id"])

        rows: list[tuple] = []
        now = datetime.now(timezone.utc)
        for _ in range(self.spec.n_purchases):
            if self.rng.random() < 0.8:
                user_id = self.rng.choice(heavy_buyers)
            else:
                user_id = self.rng.choice(user_ids)
            promo_id = self.rng.choice(promo_ids)
            partner_id = promo_to_partner[promo_id]
            amount = round(self.rng.uniform(100, 50000), 2)
            cpa = round(amount * self.rng.uniform(0.005, 0.08), 2)
            status = self.rng.choices(["confirmed", "pending", "cancelled"], weights=[0.85, 0.10, 0.05])[0]
            created_at = now - timedelta(days=self.rng.randint(0, 120), hours=self.rng.randint(0, 23))
            confirmed_at = created_at + timedelta(hours=self.rng.randint(1, 24)) if status == "confirmed" else None
            rows.append((user_id, promo_id, partner_id, amount, cpa, status, created_at, confirmed_at))

        await conn.executemany(
            """
            INSERT INTO purchases (user_id, promo_id, partner_id, amount, cpa_amount, status, created_at, confirmed_at)
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
            """,
            rows,
        )


def _slug(name: str) -> str:
    return "".join(c.lower() if c.isalnum() else "-" for c in name).strip("-")


def _new_code(used: set[str]) -> str:
    while True:
        code = "".join(secrets.choice(string.ascii_uppercase + string.digits) for _ in range(8))
        if code not in used:
            used.add(code)
            return code


def _new_phone(used: set[str], rng: random.Random) -> str:
    while True:
        phone = "+7" + "".join(str(rng.randint(0, 9)) for _ in range(10))
        if phone not in used:
            used.add(phone)
            return phone


def main() -> None:
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s: %(message)s")
    cfg = MLConfig()
    spec = GenSpec()
    asyncio.run(MockGenerator(cfg, spec).run())


if __name__ == "__main__":
    main()
