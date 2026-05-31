from __future__ import annotations

import json
import logging
import signal
import threading
import time
from typing import Iterable

from confluent_kafka import Consumer, KafkaError

from config import MLConfig
from serving.scorer import ScoringService

log = logging.getLogger(__name__)


class ScoringWorker:
    def __init__(self, cfg: MLConfig, service: ScoringService) -> None:
        self.cfg = cfg
        self.service = service
        self.stop_event = threading.Event()
        self._install_signals()

    def run_once(self) -> int:
        log.info("batch scoring start")
        n = self.service.score_all()
        log.info("batch scoring done; wallets=%d", n)
        return n

    def run_daemon(self) -> None:
        log.info("daemon mode (interval=%ds)", self.cfg.daemon_interval_sec)
        consumer_thread = threading.Thread(target=self._consume_requests, daemon=True)
        consumer_thread.start()

        last_run = 0.0
        while not self.stop_event.is_set():
            if time.time() - last_run >= self.cfg.daemon_interval_sec:
                try:
                    self.run_once()
                except Exception as e:
                    log.exception("scheduled run failed: %s", e)
                last_run = time.time()
            self.stop_event.wait(timeout=1.0)

        log.info("daemon stopping")
        self.service.close()

    def _consume_requests(self) -> None:
        consumer = Consumer({
            "bootstrap.servers": ",".join(self.cfg.kafka_brokers),
            "group.id": self.cfg.kafka_group,
            "auto.offset.reset": "latest",
            "enable.auto.commit": True,
        })
        consumer.subscribe([self.cfg.kafka_topic_requests])
        log.info("listening to %s", self.cfg.kafka_topic_requests)
        try:
            while not self.stop_event.is_set():
                msg = consumer.poll(1.0)
                if msg is None:
                    continue
                if msg.error():
                    if msg.error().code() != KafkaError._PARTITION_EOF:
                        log.warning("kafka error: %s", msg.error())
                    continue
                self._handle_request(msg.value())
        finally:
            consumer.close()

    def _handle_request(self, raw: bytes) -> None:
        try:
            doc = json.loads(raw.decode("utf-8"))
        except (UnicodeDecodeError, json.JSONDecodeError) as e:
            log.warning("malformed request: %s", e)
            return
        addresses = _coerce_addresses(doc)
        if not addresses:
            return
        log.info("on-demand scoring n=%d", len(addresses))
        try:
            self.service.score_wallets(addresses)
        except Exception as e:
            log.exception("on-demand scoring failed: %s", e)

    def _install_signals(self) -> None:
        def stop(_signum, _frame):
            log.info("signal received, shutting down")
            self.stop_event.set()
        signal.signal(signal.SIGINT, stop)
        signal.signal(signal.SIGTERM, stop)


def _coerce_addresses(doc: dict) -> list[str]:
    if isinstance(doc, dict):
        payload = doc.get("payload", doc)
        if isinstance(payload, dict):
            if "addresses" in payload and isinstance(payload["addresses"], list):
                return [str(a) for a in payload["addresses"] if a]
            if "wallet" in payload:
                return [str(payload["wallet"])]
    if isinstance(doc, list):
        return [str(a) for a in doc if a]
    return []
