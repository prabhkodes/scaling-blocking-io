# Phase 2 finding: the capacity-cliff divergence reproduces under real k8s scheduling

**Setup:** dedicated `scaling-blocking-io` minikube profile (3 CPU / 2.8GB), Docker driver, images built directly into minikube's daemon (`infra/minikube/deploy.sh`), same `mock-third-party` (lognormal median=8000ms sigma=0.5 cap=20000ms).

Scaled down from Phase 1's 100-slot budget to fit this cluster: **2 workers x 15 threads = 30 slots** on both sides (`django-uwsgi` threads, `fastapi-async` `--limit-concurrency 15` x 2 workers). Mean holding time ~9.07s → theoretical max sustainable ≈ 30 / 9.07s ≈ **3.3 req/s**. Same ratio as Phase 1: ~60% and ~180% utilization.

## Results

| App | Rate | Success | App error | p50 | p95 | p99 |
|---|---|---|---|---|---|---|
| django-uwsgi | 2 (60%) | 100% | 0% | 7.70s | 18.93s | 20.03s |
| fastapi-async | 2 (60%) | 100% | 0% | 7.24s | 18.19s | 20.01s |
| django-uwsgi | 6 (180%) | 100% | 0% | **21.27s** | 36.90s | 39.00s |
| fastapi-async | 6 (180%) | 57.3% | **42.7%** | **7.52s** | 17.54s | 20.01s |

Same story as the local docker-compose run: at baseline the two are indistinguishable, and at 180% overload they diverge exactly the same way — uwsgi's admitted requests all still succeed but p50 nearly triples (7.7s → 21.3s), fastapi holds admitted-request latency almost flat (7.2s → 7.5s) while explicitly rejecting 43% instantly. **This isn't a docker-compose networking artifact — it reproduces under real k8s Service routing, resource limits, and scheduling.**

## Where the hypothesis did NOT hold up: memory

Expected coroutines to show a clear memory advantage over OS threads at this scale (thread stacks vs. a few-KB coroutine frame). Measured with `kubectl top pods` right after the overload run:

| Pod | CPU | Memory |
|---|---|---|
| django-uwsgi (2x15=30 threads) | 6m | **78Mi** |
| fastapi-async (2 workers, event loop) | 15m | **98Mi** |

FastAPI used *more* memory here, not less. Most likely explanation: at only 30 threads, fixed per-process import overhead (FastAPI/Starlette/Pydantic's model-building machinery is meaningfully heavier to import than plain Django+requests) dominates over the thread-stack cost, and Linux allocates thread stack pages lazily — an idle blocked thread doesn't actually hold much resident memory just for existing. **The "threads cost more memory" hypothesis is untested, not confirmed, at this scale.** It needs the real 100-1500-thread range to have a chance of showing up, which this 3-CPU/2.8GB cluster can't safely host. Carrying this into Phase 3 as the specific thing to measure at realistic scale, rather than assuming the answer.

## Still open from Phase 1

Thunder-lock has still never actually been tested (env var wired up in the manifest, `THUNDER_LOCK: "false"` — Phase 3 territory). And the backlog-rejection-vs-client-timeout question from Phase 1 is still unresolved — need continuous Prometheus scraping of `listen_queue_errors` during a run, not a post-hoc snapshot, which is the next concrete piece of infra work before any more load-testing is worth doing.
