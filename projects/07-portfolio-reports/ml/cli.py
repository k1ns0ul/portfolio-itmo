from __future__ import annotations

import argparse
import json
import logging
import sys

import uvicorn

from config import LLMConfig


def _setup_logging(cfg: LLMConfig) -> None:
    logging.basicConfig(
        level=getattr(logging, cfg.log_level.upper(), logging.INFO),
        format="%(asctime)s %(levelname)s %(name)s: %(message)s",
        stream=sys.stdout,
    )


def cmd_serve(_args: argparse.Namespace, cfg: LLMConfig) -> int:
    uvicorn.run(
        "app:app",
        host=cfg.server_host,
        port=cfg.server_port,
        workers=1,
        log_config=None,
    )
    return 0


def cmd_generate_dataset(args: argparse.Namespace, cfg: LLMConfig) -> int:
    from training import DatasetBuilder
    builder = DatasetBuilder(seed=cfg.random_state)
    examples = builder.generate_examples(args.n)
    out_path = args.out
    builder.save_jsonl(examples, out_path)
    print(json.dumps({"examples": len(examples), "path": str(out_path)}, ensure_ascii=False, indent=2))
    return 0


def cmd_train(args: argparse.Namespace, cfg: LLMConfig) -> int:
    from training import QLoRATrainer
    artifacts = QLoRATrainer(cfg).train(args.dataset)
    print(json.dumps({
        "adapter_dir": str(artifacts.adapter_dir),
        "train_examples": artifacts.train_examples,
        "epochs": artifacts.epochs,
        "base_model": artifacts.base_model,
    }, ensure_ascii=False, indent=2))
    return 0


def cmd_evaluate(args: argparse.Namespace, cfg: LLMConfig) -> int:
    from inference import build_backend
    from training.evaluate import EvalPipeline, report_to_dict

    backend = build_backend(cfg.backend, cfg)
    report = EvalPipeline(cfg, backend).run(args.dataset, limit=args.limit)
    print(json.dumps(report_to_dict(report), ensure_ascii=False, indent=2))
    return 0


def cmd_generate_report(args: argparse.Namespace, cfg: LLMConfig) -> int:
    import httpx

    payload = {"address": args.address}
    url = args.api + "/api/v1/report"
    try:
        resp = httpx.post(url, json=payload, timeout=cfg.request_timeout)
        resp.raise_for_status()
    except httpx.HTTPError as e:
        print(json.dumps({"error": str(e)}, ensure_ascii=False))
        return 1
    print(json.dumps(resp.json(), ensure_ascii=False, indent=2))
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="ml")
    sub = parser.add_subparsers(dest="cmd", required=True)
    sub.add_parser("serve")

    p_ds = sub.add_parser("generate-dataset")
    p_ds.add_argument("--n", type=int, default=500)
    p_ds.add_argument("--out", default="./artifacts/dataset.jsonl")

    p_train = sub.add_parser("train")
    p_train.add_argument("--dataset", default="./artifacts/dataset.jsonl")

    p_eval = sub.add_parser("evaluate")
    p_eval.add_argument("--dataset", default="./artifacts/dataset.jsonl")
    p_eval.add_argument("--limit", type=int, default=50)

    p_rep = sub.add_parser("generate-report")
    p_rep.add_argument("--address", required=True)
    p_rep.add_argument("--api", default="http://api:8080")

    return parser


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    cfg = LLMConfig()
    _setup_logging(cfg)
    handlers = {
        "serve": cmd_serve,
        "generate-dataset": cmd_generate_dataset,
        "train": cmd_train,
        "evaluate": cmd_evaluate,
        "generate-report": cmd_generate_report,
    }
    return handlers[args.cmd](args, cfg)


if __name__ == "__main__":
    raise SystemExit(main())
