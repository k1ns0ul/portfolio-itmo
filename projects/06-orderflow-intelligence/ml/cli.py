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


def cmd_train(_args: argparse.Namespace, cfg: MLConfig) -> int:
    from training import TrainingPipeline

    pipe = TrainingPipeline(cfg)
    result = pipe.run()
    print(json.dumps({
        "rows": result.rows,
        "alpha": result.alpha,
        "xgb_accuracy": result.xgb_metrics.accuracy,
        "lstm_accuracy": result.lstm_metrics.accuracy,
        "ensemble_accuracy": result.ensemble_metrics.accuracy,
        "ensemble_f1": result.ensemble_metrics.f1_macro,
    }, indent=2, ensure_ascii=False))
    return 0


def cmd_predict(args: argparse.Namespace, cfg: MLConfig) -> int:
    from predictor import PredictionService

    svc = PredictionService(cfg)
    svc.setup()
    try:
        if args.mode == "batch":
            n = svc.run_batch()
            print(json.dumps({"written": n}, indent=2))
        else:
            svc.run_daemon()
    finally:
        svc.db.close()
    return 0


def cmd_evaluate(_args: argparse.Namespace, cfg: MLConfig) -> int:
    from training import TrainingPipeline

    pipe = TrainingPipeline(cfg)
    result = pipe.run()
    print(json.dumps({
        "ensemble_accuracy": result.ensemble_metrics.accuracy,
        "ensemble_f1": result.ensemble_metrics.f1_macro,
        "ensemble_confusion": result.ensemble_metrics.confusion,
    }, indent=2, ensure_ascii=False))
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="ml")
    sub = parser.add_subparsers(dest="cmd", required=True)
    sub.add_parser("train")
    p_pred = sub.add_parser("predict")
    p_pred.add_argument("--mode", choices=["batch", "daemon"], default="batch")
    sub.add_parser("evaluate")
    return parser


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    cfg = MLConfig()
    _setup_logging(cfg)
    handlers = {
        "train": cmd_train,
        "predict": cmd_predict,
        "evaluate": cmd_evaluate,
    }
    return handlers[args.cmd](args, cfg)


if __name__ == "__main__":
    raise SystemExit(main())
