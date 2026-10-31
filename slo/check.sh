#!/bin/sh
set -eu
VM_URL="${VM_URL:-http://127.0.0.1:8428}"
for query in \
  'up{job="timepilot-prober"} == 1' \
  'timepilot_probe_requests_total{operation="create"}' \
  '100 * sum(increase(timepilot_tasks_created_total{sli_version="1",namespace="timepilot"}[2m])) / sum(increase(timepilot_task_attempts_total{sli_version="1",namespace="timepilot",operation="create"}[2m]))' \
  '100 * sum(increase(timepilot_tasks_viewed_total{sli_version="1",namespace="timepilot"}[2m])) / sum(increase(timepilot_task_attempts_total{sli_version="1",namespace="timepilot",operation="view"}[2m]))' \
  '100 * sum(increase(timepilot_tasks_updated_total{sli_version="1",namespace="timepilot"}[2m])) / sum(increase(timepilot_task_attempts_total{sli_version="1",namespace="timepilot",operation="update"}[2m]))' \
  '100 * sum(increase(timepilot_probe_requests_total{namespace="timepilot",result="success"}[2m])) / sum(increase(timepilot_probe_requests_total{namespace="timepilot"}[2m]))'
do
  response="$(curl -fsSG "$VM_URL/api/v1/query" --data-urlencode "query=$query")"
  printf '%s\n' "$response" | jq -e '.status == "success" and (.data.result | length > 0) and all(.data.result[]; (.value[1] | tonumber) >= 0)' >/dev/null || {
    printf 'No usable data: %s\n%s\nWarm up the prober and wait for scraping/query visibility.\n' "$query" "$response" >&2
    exit 1
  }
  printf '%s\n' "$query"
  printf '%s\n' "$response" | jq -c '.data.result[] | {metric, value}'
done
