import os
import time
from contextlib import asynccontextmanager

import httpx
from fastapi import FastAPI, Response
from fastapi.responses import JSONResponse

from prometheus_client import CollectorRegistry, Counter, Gauge, Histogram, generate_latest
from prometheus_client import multiprocess

THIRD_PARTY_URL = os.environ.get("THIRD_PARTY_URL", "http://localhost:9000/delay")
THIRD_PARTY_TIMEOUT_S = float(os.environ.get("THIRD_PARTY_TIMEOUT_S", "65"))

REQUEST_LATENCY = Histogram(
    "app_request_duration_seconds",
    "End-to-end latency of the async view, including the 3rd-party call",
    buckets=(0.1, 0.25, 0.5, 1, 2, 4, 8, 12, 16, 24, 32, 48, 64),
)
IN_FLIGHT = Gauge(
    "app_requests_in_flight",
    "Requests currently awaiting the 3rd-party call",
    multiprocess_mode="livesum",
)
THIRD_PARTY_ERRORS = Counter(
    "app_third_party_errors_total",
    "Requests where the 3rd-party call failed or timed out",
)


@asynccontextmanager
async def lifespan(app: FastAPI):
    # One pooled async client per worker process, reused across requests —
    # the async-side equivalent of a requests.Session on the sync app, so
    # neither variant is paying TCP/TLS handshake cost per request.
    limits = httpx.Limits(max_connections=None, max_keepalive_connections=None)
    app.state.client = httpx.AsyncClient(timeout=THIRD_PARTY_TIMEOUT_S, limits=limits)
    yield
    await app.state.client.aclose()


app = FastAPI(lifespan=lifespan)


@app.get("/call")
async def call(response: Response):
    IN_FLIGHT.inc()
    start = time.monotonic()
    try:
        resp = await app.state.client.get(THIRD_PARTY_URL)
        resp.raise_for_status()
        payload = resp.json()
        elapsed = time.monotonic() - start
        REQUEST_LATENCY.observe(elapsed)
        return {
            "delayed_ms": payload.get("delayed_ms"),
            "elapsed_s": round(elapsed, 3),
            "worker_pid": os.getpid(),
        }
    except httpx.HTTPError as exc:
        THIRD_PARTY_ERRORS.inc()
        elapsed = time.monotonic() - start
        REQUEST_LATENCY.observe(elapsed)
        return JSONResponse({"error": str(exc), "elapsed_s": round(elapsed, 3)}, status_code=502)
    finally:
        IN_FLIGHT.dec()


@app.get("/health")
async def health():
    return Response("ok")


@app.get("/metrics")
async def metrics():
    if "PROMETHEUS_MULTIPROC_DIR" in os.environ:
        registry = CollectorRegistry()
        multiprocess.MultiProcessCollector(registry)
    else:
        from prometheus_client import REGISTRY as registry
    return Response(generate_latest(registry), media_type="text/plain; version=0.0.4")
