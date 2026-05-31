from __future__ import annotations

import logging
from contextlib import asynccontextmanager
from typing import AsyncIterator

from fastapi import FastAPI, HTTPException, Query
from pydantic import BaseModel

from antifraud import AntifraudService
from config import MLConfig
from db import PostgresDB
from recommender import RecommenderService

log = logging.getLogger(__name__)


class RecommendationItem(BaseModel):
    promo_id: int
    score: float
    reason: str
    generated_at: str


class RecommendationsResponse(BaseModel):
    user_id: int
    items: list[RecommendationItem]


class RefreshResponse(BaseModel):
    refreshed: int


class AuditResponse(BaseModel):
    report: dict


class HealthResponse(BaseModel):
    status: str
    recommender: bool
    antifraud: bool


def create_app(cfg: MLConfig | None = None) -> FastAPI:
    cfg = cfg or MLConfig()
    db = PostgresDB(cfg)
    recommender = RecommenderService(cfg, db)
    antifraud = AntifraudService(cfg, db)

    @asynccontextmanager
    async def lifespan(_app: FastAPI) -> AsyncIterator[None]:
        await db.connect()
        rec_loaded = await recommender.load()
        af_loaded = await antifraud.load()
        log.info("startup: recommender=%s, antifraud=%s", rec_loaded, af_loaded)
        try:
            yield
        finally:
            await db.close()

    app = FastAPI(title="t-vygoda ML", version="1.0", lifespan=lifespan)

    @app.get("/health", response_model=HealthResponse)
    async def health() -> HealthResponse:
        return HealthResponse(
            status="ok",
            recommender=recommender.collab.user_factors is not None,
            antifraud=antifraud.detector.iso is not None,
        )

    @app.get("/recommendations/{user_id}", response_model=RecommendationsResponse)
    async def get_recommendations(user_id: int, limit: int = Query(10, ge=1, le=100)) -> RecommendationsResponse:
        try:
            recs = await recommender.get_recommendations(user_id, limit=limit)
        except Exception as e:
            log.exception("recommendations failed for %d", user_id)
            raise HTTPException(status_code=500, detail=str(e)) from e
        items = [
            RecommendationItem(
                promo_id=r.promo_id,
                score=r.score,
                reason=r.reason,
                generated_at=r.generated_at.isoformat(),
            )
            for r in recs
        ]
        return RecommendationsResponse(user_id=user_id, items=items)

    @app.post("/recommendations/refresh", response_model=RefreshResponse)
    async def refresh_recommendations() -> RefreshResponse:
        try:
            n = await recommender.refresh_all()
        except Exception as e:
            log.exception("refresh failed")
            raise HTTPException(status_code=500, detail=str(e)) from e
        return RefreshResponse(refreshed=n)

    @app.post("/retrain/recommender")
    async def retrain_recommender() -> dict:
        try:
            return await recommender.retrain()
        except Exception as e:
            log.exception("retrain recommender failed")
            raise HTTPException(status_code=500, detail=str(e)) from e

    @app.post("/retrain/antifraud")
    async def retrain_antifraud() -> dict:
        try:
            return await antifraud.retrain()
        except Exception as e:
            log.exception("retrain antifraud failed")
            raise HTTPException(status_code=500, detail=str(e)) from e

    @app.get("/audit", response_model=AuditResponse)
    async def audit() -> AuditResponse:
        try:
            report = await antifraud.run_audit()
        except Exception as e:
            log.exception("audit failed")
            raise HTTPException(status_code=500, detail=str(e)) from e
        return AuditResponse(report=report.to_dict())

    return app


app = create_app()
