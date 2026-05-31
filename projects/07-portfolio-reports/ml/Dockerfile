FROM python:3.11-slim AS base

ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PIP_NO_CACHE_DIR=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1

WORKDIR /app

RUN apt-get update \
 && apt-get install -y --no-install-recommends \
        build-essential libgomp1 \
 && rm -rf /var/lib/apt/lists/*

COPY requirements.txt ./
RUN pip install --upgrade pip && pip install -r requirements.txt

COPY . .

ENV LLM_MODEL_DIR=/app/artifacts \
    PYTHONPATH=/app \
    LLM_BACKEND=mock

RUN mkdir -p /app/artifacts

EXPOSE 8090

ENTRYPOINT ["python", "-m", "cli"]
CMD ["serve"]
