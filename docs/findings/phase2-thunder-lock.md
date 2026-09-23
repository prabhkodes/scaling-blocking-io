# Phase 2 finding: thunder-lock, tested for the first time — and why it didn't matter here

Production never actually tested `--thunder-lock` (confirmed with the person who ran that system). First real test, on minikube, added a `timonwong/uwsgi-exporter` sidecar to scrape uwsgi's native stats into Prometheus so the listen queue could be watched live during the run instead of checked once after ([infra/minikube/manifests/20-django-uwsgi.yaml](../../infra/minikube/manifests/20-django-uwsgi.yaml), [40-monitoring.yaml](../../infra/minikube/manifests/40-monitoring.yaml)).

## First: this settles the Phase 1 open question

Same 6 req/s / 40s overload run as Phase 1/2 (2 workers x 15 threads = 30 slots, ~180% utilization), but this time with `uwsgi_listen_queue_length` and `uwsgi_listen_queue_errors` scraped every 5s throughout:

```
t+0s:  0        (idle)
t+15s: 15       (queue building)
t+30s: 56
t+45s: 98       (peak — right at the ~100 default backlog ceiling)
t+60s: 54       (draining)
t+85s: 0        (idle)

listen_queue_errors throughout: 0
```

The queue got close (98/100) but **never actually overflowed** — confirms the Phase 1/2 "unreachable" results were client-timeout artifacts, not real kernel backlog rejection. A slightly higher rate or longer sustained run would very plausibly tip this over; worth deliberately targeting in Phase 3.

## Thunder-lock on vs. off: no measurable difference at this scale

Identical 6 req/s / 40s run, `THUNDER_LOCK=false` vs `true`, otherwise unchanged:

| | p50 | p90 | p95 | p99 | max | queue peak | queue errors |
|---|---|---|---|---|---|---|---|
| thunder-lock off | 21.66s | 36.00s | 38.05s | 42.28s | 47.28s | 98 | 0 |
| thunder-lock on | 21.63s | 34.85s | 38.46s | 42.75s | 46.27s | 97 | 0 |

Within noise. **This is expected, not a wasted test** — re-reading uWSGI's own docs on the mechanism clarifies why: thunder-lock serializes `accept()` across worker *processes* to stop every idle process from waking up (and wasting CPU/context-switch time) when a connection arrives but only one can take it. That waste is proportional to how many processes are sitting *idle* competing for a *sparse* trickle of connections. In this test the opposite was true — all worker capacity was saturated (queue depth 98/100), so there was no idle-process race to serialize in the first place. Thunder-lock fixes wasted wakeup CPU; it does not fix a genuinely full thread pool queueing real work, which is a capacity problem, not a coordination problem.

Also: production ran **15 workers**, this test ran **2**. Thunder-lock's savings scale with worker count (each spurious wakeup wastes up to `workers - 1` processes' worth of CPU per connection) — 2 workers has very little waste to eliminate regardless of load regime. To actually see whether thunder-lock would have helped production, Phase 3 needs to retest at ~15 workers specifically **under light-to-moderate, bursty load** (many idle workers, occasional connections) rather than sustained saturation — the opposite regime from this test.

## Updated picture for Phase 3

Three things now explicitly require realistic scale (100+ threads, ~15 workers) to actually resolve, all currently either null or unconfirmed at minikube scale:
1. Memory-per-model hypothesis (coroutines vs. thread stacks) — inconclusive at 30 threads (see [phase2-minikube-reproduction.md](phase2-minikube-reproduction.md))
2. Thunder-lock's real benefit — needs 15 workers + idle/bursty load, not 2 workers + saturation
3. Real backlog overflow — queue got to 98/100 but never tipped over; needs a deliberately harder push
