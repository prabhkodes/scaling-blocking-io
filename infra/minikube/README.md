# infra/minikube

Local k8s manifests for fast-iteration testing of all three app variants plus the Prometheus/Grafana/Loki stack, before anything runs on GKE.

Cluster: run `./setup.sh` to bring up a dedicated `scaling-blocking-io` minikube profile (3 CPU / 2.8GB, metrics-server enabled). Kept as its own profile rather than reusing the machine's default `minikube` profile, which runs unrelated long-lived work.

Status: cluster bootstrap done. App/monitoring manifests not yet implemented (Phase 2).
