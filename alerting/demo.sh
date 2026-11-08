#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
metrics_url="${METRICS_URL:-http://127.0.0.1:9090}"
case "${1:-}" in
  business) ./slo/mode.sh down ;;
  runtime)
    curl -fsS -X POST "$metrics_url/demo/runtime" -H 'Content-Type: application/json' --data '{"enabled":true}'
    printf '\n' ;;
  healthy) ./slo/mode.sh healthy ;;
  *) echo 'Usage: ./alerting/demo.sh business|runtime|healthy' >&2; exit 1 ;;
esac
