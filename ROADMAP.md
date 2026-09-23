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
- [ ] Define the mock 3rd-party latency distribution (lognormal, ~median + long tail, matching "8s+" observed)
- [ ] Define metrics to capture per run (throughput, p50/p95/p99, error taxonomy, CPU, RSS, context switches, uWSGI stats)

## Phase 1 — Local prototype
- [ ] `apps/mock-third-party`: configurable-latency dependency (sleep distribution via env/query param)
- [ ] `apps/django-uwsgi`: sync baseline, instrumented, configurable workers/threads/thunder-lock
- [ ] `apps/fastapi-async`: ASGI variant using an async HTTP client
- [ ] `loadtester`: Go open-loop generator (token-bucket rate limiter, per-request latency recording to Parquet/CSV, live Prometheus export)
- [ ] Validate all three apps + load tester run correctly on local machine (`docker compose`)

## Phase 2 — Minikube
- [ ] k8s manifests (Deployment, Service, resource requests/limits) for all three app variants
- [ ] Prometheus + Grafana + Loki stack on minikube
- [ ] First end-to-end sweep on minikube: reproduce the capacity cliff at small scale
- [ ] Thunder-lock on/off/SO_REUSEPORT comparison

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
**Current phase: 0 → 1**
