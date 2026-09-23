# mock-third-party

Configurable-latency dependency standing in for the real 3rd-party API. Latency drawn from a lognormal distribution (tunable median/tail) to match the "8s+, long-tailed" behavior observed in production, plus a slowdown-injection mode for bulkhead/circuit-breaker testing.

Status: not yet implemented (Phase 1).
