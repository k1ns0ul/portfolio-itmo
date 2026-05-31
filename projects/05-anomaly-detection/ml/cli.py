from __future__ import annotations

import argparse
import json
import logging
import sys

from config import MLConfig


def _setup_logging(cfg: MLConfig) -> None:
    logging.basicConfig(
        level=getattr(logging, cfg.log_level.upper(), logging.INFO),
        format="%(asctime)s %(levelname)s %(name)s: %(message)s",
        stream=sys.stdout,
    )


def cmd_worker(_args: argparse.Namespace, cfg: MLConfig) -> int:
    from worker import InferenceWorker
    InferenceWorker(cfg).run()
    return 0


def cmd_train(args: argparse.Namespace, cfg: MLConfig) -> int:
    from training import TrainingPipeline
    result = TrainingPipeline(cfg).run(csv_path=args.csv)
    print(json.dumps({
        "rows": result.rows,
        "iforest_metrics": result.iforest_metrics,
        "autoencoder_metrics": result.autoencoder_metrics,
        "threshold": result.threshold,
    }, indent=2, ensure_ascii=False))
    return 0


def cmd_evaluate(args: argparse.Namespace, cfg: MLConfig) -> int:
    import numpy as np
    import pandas as pd

    from training import TrainingPipeline
    from training.pipeline import FEATURE_COLUMNS

    df = pd.read_csv(args.csv)
    X = df[FEATURE_COLUMNS].to_numpy(dtype=np.float32)
    metrics = TrainingPipeline(cfg).evaluate(X)
    print(json.dumps(metrics, indent=2, ensure_ascii=False))
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="ml")
    sub = parser.add_subparsers(dest="cmd", required=True)
    sub.add_parser("worker")
    p_train = sub.add_parser("train")
    p_train.add_argument("--csv", default=None, help="optional CSV path to bootstrap training data")
    p_eval = sub.add_parser("evaluate")
    p_eval.add_argument("--csv", required=True)
    return parser


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    cfg = MLConfig()
    _setup_logging(cfg)
    handlers = {"worker": cmd_worker, "train": cmd_train, "evaluate": cmd_evaluate}
    return handlers[args.cmd](args, cfg)


if __name__ == "__main__":
    raise SystemExit(main())
