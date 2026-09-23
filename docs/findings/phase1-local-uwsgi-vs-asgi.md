# Phase 1 finding: sync thread-pool vs. async load-shedding under the same overload

**Setup:** local macOS, Docker Compose, `mock-third-party` latency lognormal(median=8000ms, sigma=0.5, cap 20000ms).
Both apps sized to the **same theoretical concurrency budget: 100 in-flight requests.**

- `django-uwsgi`: 4 workers x 25 threads = 100 OS threads (`--enable-threads`, no thunder-lock)
- `fastapi-async`: 4 uvicorn workers, `--limit-concurrency 25` each = 100 total admitted requests

Mean holding time at these lognormal params ≈ 9.07s, so theoretical max sustainable throughput ≈ 100 / 9.07s ≈ **11 req/s**. Two runs per app, open-loop load (`loadtester`), 40s each:

| Run | Rate | Utilization |
|---|---|---|
| within-capacity | 6 req/s | ~54% |
| over-capacity | 18 req/s | ~163% |

## Results

| App | Rate | Success | App error | Unreachable | p50 | p95 | p99 | max |
|---|---|---|---|---|---|---|---|---|
| django-uwsgi | 6 | 100.0% | 0% | 0% | 8.46s | 18.84s | 20.01s | 20.01s |
| fastapi-async | 6 | 99.6% | 0.4% | 0% | 8.22s | 18.60s | 20.01s | 20.01s |
| django-uwsgi | 18 | 99.0% | 0% | **1.0%** | **17.09s** | 35.44s | 47.50s | 60.12s |
| fastapi-async | 18 | 60.1% | **39.9%** | 0% | **7.92s** | 17.78s | 20.01s | 20.02s |

Raw per-request CSVs: `analysis/data/local-run1/`.

## Interpretation

At matched baseline load (6 req/s, ~54% utilization) the two are indistinguishable — p50/p95/p99 line up almost exactly, as they should when nobody's queueing.

At 163% of capacity they fail in **opposite ways**:

- **django-uwsgi queues.** Requests that eventually succeed still succeed (99%), but they wait behind whichever of the 100 threads frees up first — p50 roughly doubles (8.5s → 17.1s) and p99 more than doubles into the tens of seconds, with the max run bumping the load tester's own 65s client timeout. This is unbounded-queue behavior: the thread pool has no concept of "too full," it just gets slower for everyone, including requests that arrived when the system was still healthy.
- **fastapi-async sheds load immediately.** `--limit-concurrency` rejects anything past the 100th in-flight request with an instant 503 (avg ~3.9ms to fail) — but the 60% that *do* get admitted see almost no latency degradation at all (p50 7.92s vs. 8.22s baseline). This is bounded-queue / admission-control behavior: the system protects requests it has already accepted at the cost of explicitly refusing the rest.

Neither is unconditionally "better" — it's a real design tradeoff (unbounded queueing gives every client a chance at the cost of collective latency collapse; load shedding keeps admitted requests fast at the cost of an explicit failure rate) — but it's a sharp, concrete illustration of *why* that tradeoff exists, not just an assertion of it.

## Open question — do not yet claim this reproduces the original "no status code" mystery

Django's 1.0% "unreachable" (connection-level failure, no HTTP status) at 18 req/s is the closest thing so far to the original production symptom. But `uwsgi`'s own stats server reported **`listen_queue_errors: 0`** for this run (a cumulative counter, so trustworthy even checked after the run ended) — meaning the kernel-level listen backlog was never actually exceeded. The more likely explanation for those specific failures is mundane: some requests were still queued behind busy threads when the *load tester's own* 65s client timeout fired, which looks identical to "unreachable" in the data but isn't the same failure as backlog rejection.

To actually distinguish these, Phase 2 needs to either (a) push utilization far higher / for longer so real backlog rejection has a chance to occur, or (b) continuously scrape `listen_queue`/`listen_queue_errors` into Prometheus during the run instead of checking once afterward — a one-off `curl` only sees the instant you happened to check, not the spike. Recording this as the reason Phase 2 wires up real Prometheus scraping rather than ad hoc stats checks.
