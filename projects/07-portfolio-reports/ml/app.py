from __future__ import annotations

import logging
from contextlib import asynccontextmanager
from typing import Any, AsyncIterator

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field

from config import LLMConfig
from inference import LLMBackend, build_backend
from prompts import build_prompt, extract_summary

log = logging.getLogger(__name__)


class GenerateRequest(BaseModel):
    portfolio: dict[str, Any] = Field(default_factory=dict)


class GenerateResponse(BaseModel):
    text: str
    summary: str
    source: str


class HealthResponse(BaseModel):
    status: str
    backend: str
    ready: bool


def create_app(cfg: LLMConfig | None = None) -> FastAPI:
    cfg = cfg or LLMConfig()
    state: dict[str, LLMBackend | None] = {"backend": None}

    @asynccontextmanager
    async def lifespan(_app: FastAPI) -> AsyncIterator[None]:
        try:
            state["backend"] = build_backend(cfg.backend, cfg)
            log.info("backend=%s loaded", state["backend"].name)
        except Exception as e:
            log.exception("backend init failed: %s", e)
            state["backend"] = None
        try:
            yield
        finally:
            if state["backend"] is not None:
                state["backend"].close()

    app = FastAPI(title="portfolio-reports llm", version="1.0", lifespan=lifespan)

    @app.get("/health", response_model=HealthResponse)
    async def health() -> HealthResponse:
        backend = state["backend"]
        ready = bool(backend and backend.ready())
        name = backend.name if backend else "none"
        return HealthResponse(status="ok" if ready else "degraded", backend=name, ready=ready)

    @app.post("/generate", response_model=GenerateResponse)
    async def generate(req: GenerateRequest) -> GenerateResponse:
        backend = state["backend"]
        if backend is None:
            raise HTTPException(status_code=503, detail="backend not ready")
        prompt = build_prompt(req.portfolio)
        try:
            text = backend.generate(
                prompt,
                max_tokens=cfg.max_tokens,
                temperature=cfg.temperature,
                top_p=cfg.top_p,
            )
        except Exception as e:
            log.exception("generation failed: %s", e)
            raise HTTPException(status_code=500, detail=str(e)) from e
        return GenerateResponse(
            text=text,
            summary=extract_summary(text),
            source=backend.name,
        )

    @app.post("/generate/batch", response_model=list[GenerateResponse])
    async def generate_batch(items: list[GenerateRequest]) -> list[GenerateResponse]:
        backend = state["backend"]
        if backend is None:
            raise HTTPException(status_code=503, detail="backend not ready")
        out: list[GenerateResponse] = []
        for req in items:
            prompt = build_prompt(req.portfolio)
            try:
                text = backend.generate(
                    prompt,
                    max_tokens=cfg.max_tokens,
                    temperature=cfg.temperature,
                    top_p=cfg.top_p,
                )
            except Exception as e:
                log.exception("batch item failed: %s", e)
                raise HTTPException(status_code=500, detail=str(e)) from e
            out.append(GenerateResponse(text=text, summary=extract_summary(text), source=backend.name))
        return out

    return app


app = create_app()
