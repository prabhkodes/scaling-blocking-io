# loadtester

Open-loop Go load generator. Issues requests on a fixed schedule (token-bucket rate limiter) regardless of response time, to avoid Coordinated Omission (a closed-loop generator silently hides tail latency once the server saturates). Records raw per-request latency to Parquet/CSV and exports live Prometheus metrics.

Status: not yet implemented (Phase 1).
