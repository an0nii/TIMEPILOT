#!/bin/sh
set -eu
case "${1:-}" in
  healthy|degraded|down) mode="$1" ;;
  *) echo 'Usage: ./slo/mode.sh healthy|degraded|down' >&2; exit 1 ;;
esac
curl -fsS -X POST "${METRICS_URL:-http://127.0.0.1:9090}/demo/mode" \
  -H 'Content-Type: application/json' --data "{\"mode\":\"$mode\"}"
printf '\n'
