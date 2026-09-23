# scaling-blocking-io

**How many threads does a web tier need when the thing it's waiting on is slow — and is autoscaling even the right tool for that problem?**

This is an independent, from-scratch study. It's motivated by a real production investigation I ran previously (Django/uWSGI behind Kubernetes, calling a 3rd-party API with **8+ second** response times, at **200–600+ RPS**), but contains no proprietary code, data, or dashboards from that employer — everything here (apps, load generator, infra, numbers) is rebuilt independently against synthetic workloads that reproduce the same *shape* of problem.

## The question that started this

Production ran uWSGI with **15 worker processes × 100 threads = 1,500 concurrent request slots per pod**, fronting a synchronous Django view that blocked on a 3rd-party HTTP call. `--thunder-lock` was never enabled. Availability dashboards periodically showed a category of failure distinct from 502/503: requests with **no status code at all** — the connection never reached the application.

Doing the capacity math after the fact was uncomfortable:

```
Little's Law: concurrency required = arrival_rate × time_per_request

At 200 RPS,  8s wait  →  1,600 concurrent slots required
At 600 RPS,  8s wait  →  4,800 concurrent slots required

Single pod capacity (15 workers × 100 threads)  =  1,500 slots
```

At the **low end** of observed traffic, a single pod's entire thread budget was already spoken for — before accounting for thundering-herd waste, the 100-connection kernel listen backlog, or GIL overhead on the non-blocked portion of each request. And the time to exhaust that budget from empty (`capacity ÷ arrival_rate`) is **2.5–7.5 seconds** — well inside the reaction time of a default Kubernetes HPA loop. That's a plausible root cause for the "no status code" failures: kernel-level backlog rejection during a capacity cliff, invisible at the WSGI/application layer entirely.

None of this was tested against a hypothesis at the time. This repo is that test.

## What this repo actually investigates

1. **Reproduce the capacity cliff** — a mock 3rd-party dependency with configurable (and realistic, long-tailed) latency, ramp RPS past the theoretical exhaustion point, and confirm whether kernel-backlog rejection vs. app-level 502/503 are actually distinct, separately-caused failure modes.
2. **Thundering herd, actually measured** — uWSGI's `--thunder-lock` was never tested in production. Compare no-mitigation vs. `--thunder-lock` vs. kernel-level `SO_REUSEPORT`, at realistic worker counts, on context-switch rate and p99 latency.
3. **Thread-per-request vs. decoupled async** — the same endpoint as uWSGI/Django (sync, thread-per-request) vs. FastAPI/uvicorn (async, event-loop) vs. a queue-decoupled variant. Measured on memory per concurrent request and resilience to a 3rd-party slowdown injection — not just throughput.
4. **HPA reaction time vs. exhaustion window** — does autoscaling actually help when the failure mode is faster than the control loop, and if not, what does (headroom, decoupling, bulkheads/circuit breakers)?

## Repo layout

```
apps/
  django-uwsgi/      sync thread-per-request baseline (the original architecture)
  fastapi-async/      async/ASGI gateway variant
  mock-third-party/    configurable-latency dependency (the thing being "waited on")
loadtester/            open-loop Go load generator (avoids Coordinated Omission)
infra/
  minikube/            local k8s manifests for fast iteration
  gke/                 cluster + workload config for cloud-scale runs
  monitoring/          Prometheus / Grafana / Loki stack
docs/findings/         write-up per experiment, with real measured numbers
analysis/              notebooks turning raw run data into the charts in docs/findings
```

## Status

See [ROADMAP.md](ROADMAP.md) for current phase and progress.
