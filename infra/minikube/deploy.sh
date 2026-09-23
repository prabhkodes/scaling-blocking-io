#!/usr/bin/env bash
# Builds all three app images directly inside minikube's own Docker daemon
# (so no registry push/pull is needed) and applies the manifests.
set -euo pipefail

PROFILE="scaling-blocking-io"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

eval "$(minikube -p "$PROFILE" docker-env)"

docker build -t scaling-blocking-io-mock-third-party:local "$ROOT/apps/mock-third-party"
docker build -t scaling-blocking-io-django-uwsgi:local "$ROOT/apps/django-uwsgi"
docker build -t scaling-blocking-io-fastapi-async:local "$ROOT/apps/fastapi-async"

kubectl --context="$PROFILE" apply -f "$ROOT/infra/minikube/manifests/"

echo
echo "waiting for rollout..."
kubectl --context="$PROFILE" -n scaling-study rollout status deployment/mock-third-party --timeout=60s
kubectl --context="$PROFILE" -n scaling-study rollout status deployment/django-uwsgi --timeout=60s
kubectl --context="$PROFILE" -n scaling-study rollout status deployment/fastapi-async --timeout=60s

kubectl --context="$PROFILE" -n scaling-study get pods -o wide
