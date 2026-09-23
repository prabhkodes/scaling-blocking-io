#!/usr/bin/env bash
# Builds the uwsgi invocation from env vars so the worker/thread/thunder-lock
# axis can be swept without rebuilding the image — this is the exact
# production config surface (15 workers x 100 threads, thunder-lock never
# set) that motivated this whole study.
set -euo pipefail

WORKERS="${WORKERS:-4}"
THREADS="${THREADS:-4}"
PORT="${PORT:-8000}"
THUNDER_LOCK="${THUNDER_LOCK:-false}"

export PROMETHEUS_MULTIPROC_DIR="${PROMETHEUS_MULTIPROC_DIR:-/tmp/prometheus-multiproc}"
rm -rf "$PROMETHEUS_MULTIPROC_DIR"
mkdir -p "$PROMETHEUS_MULTIPROC_DIR"

ARGS=(
  --http ":${PORT}"
  --wsgi-file app/wsgi.py
  --workers "${WORKERS}"
  --threads "${THREADS}"
  --enable-threads
  --master
  --stats "0.0.0.0:9191"
  --stats-http
)

if [ "${THUNDER_LOCK}" = "true" ]; then
  ARGS+=(--thunder-lock)
fi

echo "starting uwsgi: workers=${WORKERS} threads=${THREADS} thunder_lock=${THUNDER_LOCK}"
exec uwsgi "${ARGS[@]}"
