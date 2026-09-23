# Roadmap

Tracking progress phase by phase. Each phase has a matching GitHub issue for finer-grained tasks:
[#1 Phase 0](https://github.com/prabhkodes/scaling-blocking-io/issues/1) ·
[#2 Phase 1](https://github.com/prabhkodes/scaling-blocking-io/issues/2) ·
[#3 Phase 2](https://github.com/prabhkodes/scaling-blocking-io/issues/3) ·
[#4 Phase 3](https://github.com/prabhkodes/scaling-blocking-io/issues/4) ·
[#5 Phase 4](https://github.com/prabhkodes/scaling-blocking-io/issues/5)

## Phase 0 — Hypothesis & design
- [x] Capacity math from real production numbers (Little's Law, exhaustion window)
- [x] Identify the four experiment threads (capacity cliff, thundering herd, sync-vs-async, HPA lag)
- [x] Define the mock 3rd-party latency distribution (lognormal, median=8000ms sigma=0.5, matching "8s+" observed)
- [x] Define metrics to capture per run (throughput, p50/p95/p99, error taxonomy — CPU/RSS/context-switches deferred to Phase 2, need continuous scraping not one-off checks)

## Phase 1 — Local prototype ✅
- [x] `apps/mock-third-party`: configurable-latency dependency (lognormal via env, `?ms=` override for deterministic testing)
- [x] `apps/django-uwsgi`: sync baseline, instrumented, configurable workers/threads/thunder-lock, pooled `requests.Session`, multiprocess Prometheus metrics
- [x] `apps/fastapi-async`: ASGI variant, pooled `httpx.AsyncClient`, `--limit-concurrency` as admission control
- [x] `loadtester`: Go open-loop generator (token-bucket via ticker, per-request CSV, success/app-error/unreachable classification) — Prometheus live export deferred to Phase 2
- [x] Validate all three apps + load tester locally via `docker compose` — first real comparison run, see [docs/findings/phase1-local-uwsgi-vs-asgi.md](docs/findings/phase1-local-uwsgi-vs-asgi.md)

## Phase 2 — Minikube ✅
- [x] k8s manifests (Deployment, Service, resource requests/limits) for all three app variants
- [x] Prometheus + Grafana on minikube (`timonwong/uwsgi-exporter` sidecar translates uwsgi's native stats — `listen_queue_length`, `listen_queue_errors`, per-worker RSS — into Prometheus). Loki skipped — no structured logging pipeline needed yet.
- [x] First end-to-end sweep on minikube: reproduce the capacity cliff at small scale — confirmed, see [docs/findings/phase2-minikube-reproduction.md](docs/findings/phase2-minikube-reproduction.md). Memory-per-model hypothesis did NOT hold at 30-thread scale — open question carried to Phase 3.
- [x] Thunder-lock on/off comparison — tested for the first time (production never did). **No measurable difference at this scale** — see [docs/findings/phase2-thunder-lock.md](docs/findings/phase2-thunder-lock.md) for why (thunder-lock fixes idle-process wakeup waste, not a saturated thread pool; also only 2 workers here vs. production's 15). Live queue-depth tracking also settled the Phase 1 open question: queue peaked at 98/100 but `listen_queue_errors` stayed 0 — confirmed those were client-timeout artifacts, not real backlog rejection. SO_REUSEPORT comparison not yet done.

## Phase 3 — GKE (blocked on: confirming a non-production GCP project/account)
- [ ] Provision GKE cluster (spot/preemptible nodes, autoscale-to-zero, cost guardrails)
- [ ] Argo Workflows for automated matrix sweeps
- [ ] Full sync-vs-async-vs-queue comparison at realistic 200–600 RPS scale
- [ ] HPA reaction-time measurement against the exhaustion window

## Phase 4 — Analysis & write-up
- [ ] Turn raw run data into charts (analysis/)
- [ ] docs/findings write-up per experiment thread
- [ ] Final blog post / portfolio piece tying all four threads back to the original mystery

---
**Current phase: 2 → 3** (blocked on GCP account confirmation)
