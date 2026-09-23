import os
import time

import requests
from django.conf import settings
from django.http import HttpResponse, JsonResponse

from prometheus_client import CollectorRegistry, Counter, Gauge, Histogram, generate_latest
from prometheus_client import multiprocess

# uWSGI runs multiple worker *processes* sharing this codebase but not memory,
# so a plain in-process prometheus_client registry would only ever reflect
# whichever single worker happened to serve the /metrics scrape. Multiprocess
# mode aggregates across workers via a shared mmap directory instead.
# Requires PROMETHEUS_MULTIPROC_DIR to be set (see Dockerfile/run.sh) *before*
# prometheus_client is imported anywhere in the process.

REQUEST_LATENCY = Histogram(
    "app_request_duration_seconds",
    "End-to-end latency of the blocking view, including the 3rd-party call",
    buckets=(0.1, 0.25, 0.5, 1, 2, 4, 8, 12, 16, 24, 32, 48, 64),
)
IN_FLIGHT = Gauge(
    "app_requests_in_flight",
    "Requests currently blocked waiting on the 3rd-party call",
    multiprocess_mode="livesum",
)
THIRD_PARTY_ERRORS = Counter(
    "app_third_party_errors_total",
    "Requests where the 3rd-party call failed or timed out",
)

# One pooled Session per *process* (shared across that process's uwsgi
# threads — urllib3's connection pool is thread-safe). Default pool_maxsize
# is only 10; with up to ~100 threads/worker hammering it concurrently,
# an undersized pool would silently discard and reopen connections past
# capacity, paying a TCP/TLS handshake per request and making the sync app
# look artificially slower than the async variant for reasons that have
# nothing to do with the thread/GIL model actually under test.
_adapter = requests.adapters.HTTPAdapter(
    pool_connections=1,
    pool_maxsize=int(os.environ.get("POOL_MAXSIZE", "256")),
)
_session = requests.Session()
_session.mount("http://", _adapter)
_session.mount("https://", _adapter)


def blocking_view(request):
    IN_FLIGHT.inc()
    start = time.monotonic()
    try:
        resp = _session.get(settings.THIRD_PARTY_URL, timeout=settings.THIRD_PARTY_TIMEOUT_S)
        resp.raise_for_status()
        payload = resp.json()
        elapsed = time.monotonic() - start
        REQUEST_LATENCY.observe(elapsed)
        return JsonResponse({
            "delayed_ms": payload.get("delayed_ms"),
            "elapsed_s": round(elapsed, 3),
            "worker_pid": os.getpid(),
        })
    except requests.RequestException as exc:
        THIRD_PARTY_ERRORS.inc()
        elapsed = time.monotonic() - start
        REQUEST_LATENCY.observe(elapsed)
        return JsonResponse({"error": str(exc), "elapsed_s": round(elapsed, 3)}, status=502)
    finally:
        IN_FLIGHT.dec()


def health(request):
    return HttpResponse("ok")


def metrics(request):
    registry = CollectorRegistry()
    multiprocess.MultiProcessCollector(registry)
    return HttpResponse(generate_latest(registry), content_type="text/plain; version=0.0.4")
