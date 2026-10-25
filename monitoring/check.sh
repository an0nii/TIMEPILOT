#!/bin/sh
set -eu
# Run with VictoriaMetrics port-forward active.
VM_URL="${VM_URL:-http://127.0.0.1:8428}"
command -v jq >/dev/null 2>&1 || { echo 'Install jq first: brew install jq' >&2; exit 1; }
# Account for initial Kubernetes discovery and VictoriaMetrics' default 30s query latency.
deadline=$(( $(date +%s) + 90 ))

for query in \
  'up{job="timepilot-web"} == 1' \
  'up{job="timepilot-database"} == 1' \
  'go_goroutines{job="timepilot-web"}' \
  'pg_up{job="timepilot-database"} == 1' \
  'pg_stat_database_xact_commit{job="timepilot-database"}' \
  'timepilot_tasks_created_total' \
  'timepilot_tasks_viewed_total' \
  'timepilot_tasks_updated_total' \
  'timepilot_tasks_deleted_total'
do
  while :; do
    response="$(curl -fsSG "$VM_URL/api/v1/query" --data-urlencode "query=$query")"
    if printf '%s\n' "$response" | jq -e '.status == "success" and (.data.result | length > 0)' >/dev/null; then
      break
    fi
    if [ "$(date +%s)" -ge "$deadline" ]; then
      printf 'FAIL: %s\n%s\n' "$query" "$response" >&2
      exit 1
    fi
    printf 'Waiting for discovery/scrape/query visibility: %s\n' "$query"
    sleep 5
  done
  printf 'OK: %s\n' "$query"
  printf '%s\n' "$response" | jq -c '.data.result[] | {metric, value}'
done
