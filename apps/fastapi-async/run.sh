#!/usr/bin/env bash
set -euo pipefail

WORKERS="${WORKERS:-4}"
PORT="${PORT:-8000}"

export PROMETHEUS_MULTIPROC_DIR="${PROMETHEUS_MULTIPROC_DIR:-/tmp/prometheus-multiproc}"
rm -rf "$PROMETHEUS_MULTIPROC_DIR"
mkdir -p "$PROMETHEUS_MULTIPROC_DIR"

ARGS=(main:app --host 0.0.0.0 --port "${PORT}" --workers "${WORKERS}")

if [ -n "${LIMIT_CONCURRENCY:-}" ]; then
  ARGS+=(--limit-concurrency "${LIMIT_CONCURRENCY}")
fi

echo "starting uvicorn: workers=${WORKERS} limit_concurrency=${LIMIT_CONCURRENCY:-none}"
exec uvicorn "${ARGS[@]}"
