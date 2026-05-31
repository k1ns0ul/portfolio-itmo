from __future__ import annotations

import argparse
import asyncio
import json
import logging
import sys

import uvicorn

from config import MLConfig


def _setup_logging(cfg: MLConfig) -> None:
    logging.basicConfig(
        level=getattr(logging, cfg.log_level.upper(), logging.INFO),
        format="%(asctime)s %(levelname)s %(name)s: %(message)s",
        stream=sys.stdout,
    )


def cmd_serve(_args: argparse.Namespace, cfg: MLConfig) -> int:
    uvicorn.run(
        "app:app",
        host=cfg.server_host,
        port=cfg.server_port,
        workers=1,
        log_config=None,
    )
    return 0


def cmd_train_recommender(_args: argparse.Namespace, cfg: MLConfig) -> int:
    from training.train_recommender import run
    metrics = asyncio.run(run(cfg))
    print(json.dumps(metrics, indent=2, ensure_ascii=False))
    return 0


def cmd_train_antifraud(_args: argparse.Namespace, cfg: MLConfig) -> int:
    from training.train_antifraud import run
    metrics = asyncio.run(run(cfg))
    print(json.dumps(metrics, indent=2, ensure_ascii=False))
    return 0


def cmd_audit(_args: argparse.Namespace, cfg: MLConfig) -> int:
    from antifraud import AntifraudService
    from db import PostgresDB

    async def runner() -> dict:
        db = PostgresDB(cfg)
        await db.connect()
        svc = AntifraudService(cfg, db)
        await svc.load()
        try:
            report = await svc.run_audit()
            return report.to_dict()
        finally:
            await db.close()

    report = asyncio.run(runner())
    print(json.dumps(report, indent=2, ensure_ascii=False))
    return 0


def cmd_generate_data(_args: argparse.Namespace, cfg: MLConfig) -> int:
    from mockdata.generator import GenSpec, MockGenerator

    async def runner() -> dict:
        return await MockGenerator(cfg, GenSpec()).run()

    stats = asyncio.run(runner())
    print(json.dumps(stats, indent=2, ensure_ascii=False))
    return 0


def cmd_refresh(_args: argparse.Namespace, cfg: MLConfig) -> int:
    from db import PostgresDB
    from recommender import RecommenderService

    async def runner() -> int:
        db = PostgresDB(cfg)
        await db.connect()
        svc = RecommenderService(cfg, db)
        await svc.load()
        try:
            return await svc.refresh_all()
        finally:
            await db.close()

    n = asyncio.run(runner())
    print(json.dumps({"refreshed": n}))
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="ml")
    sub = parser.add_subparsers(dest="cmd", required=True)
    sub.add_parser("serve")
    sub.add_parser("train-recommender")
    sub.add_parser("train-antifraud")
    sub.add_parser("audit")
    sub.add_parser("generate-data")
    sub.add_parser("refresh-recommendations")
    return parser


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    cfg = MLConfig()
    _setup_logging(cfg)
    handlers = {
        "serve": cmd_serve,
        "train-recommender": cmd_train_recommender,
        "train-antifraud": cmd_train_antifraud,
        "audit": cmd_audit,
        "generate-data": cmd_generate_data,
        "refresh-recommendations": cmd_refresh,
    }
    return handlers[args.cmd](args, cfg)


if __name__ == "__main__":
    raise SystemExit(main())
