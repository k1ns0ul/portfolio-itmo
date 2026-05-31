from __future__ import annotations

import argparse
import logging
import sys
from pathlib import Path

from config import MLConfig


def _setup_logging(cfg: MLConfig) -> None:
    logging.basicConfig(
        level=getattr(logging, cfg.log_level.upper(), logging.INFO),
        format="%(asctime)s %(levelname)s %(name)s: %(message)s",
        stream=sys.stdout,
    )


def cmd_train(_args: argparse.Namespace, cfg: MLConfig) -> int:
    from training import TrainingPipeline, write_report
    pipe = TrainingPipeline(cfg)
    result = pipe.run()
    text = write_report(cfg.model_dir)
    print(text)
    return 0 if result.ensemble_metrics.accuracy > 0 else 1


def cmd_score(args: argparse.Namespace, cfg: MLConfig) -> int:
    from serving import ScoringService, ScoringWorker
    svc = ScoringService(cfg)
    worker = ScoringWorker(cfg, svc)
    try:
        if args.mode == "batch":
            worker.run_once()
        else:
            worker.run_daemon()
    finally:
        svc.close()
    return 0


def cmd_score_tokens(_args: argparse.Namespace, cfg: MLConfig) -> int:
    from token_scoring import TokenScorer
    scorer = TokenScorer(cfg)
    try:
        n = scorer.score_all()
        logging.getLogger(__name__).info("token scoring done, n=%d", n)
    finally:
        scorer.close()
    return 0


def cmd_report(_args: argparse.Namespace, cfg: MLConfig) -> int:
    from training import write_report
    text = write_report(cfg.model_dir)
    print(text)
    return 0


def cmd_evaluate(_args: argparse.Namespace, cfg: MLConfig) -> int:
    from training import write_report
    path = Path(cfg.model_dir) / "training_result.json"
    if not path.exists():
        logging.getLogger(__name__).error("no training_result.json in %s", cfg.model_dir)
        return 1
    print(write_report(cfg.model_dir, dst=False))
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="ml")
    sub = parser.add_subparsers(dest="cmd", required=True)

    sub.add_parser("train")

    p_score = sub.add_parser("score")
    p_score.add_argument("--mode", choices=["batch", "daemon"], default="batch")

    sub.add_parser("score-tokens")
    sub.add_parser("report")
    sub.add_parser("evaluate")
    return parser


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    cfg = MLConfig()
    _setup_logging(cfg)
    handlers = {
        "train": cmd_train,
        "score": cmd_score,
        "score-tokens": cmd_score_tokens,
        "report": cmd_report,
        "evaluate": cmd_evaluate,
    }
    return handlers[args.cmd](args, cfg)


if __name__ == "__main__":
    raise SystemExit(main())
