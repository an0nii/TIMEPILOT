#!/bin/sh
set -eu
# Run with the application port-forward active. Only the task created here is removed.
APP_URL="${APP_URL:-http://127.0.0.1:8080}"
command -v jq >/dev/null 2>&1 || { echo 'Install jq first: brew install jq' >&2; exit 1; }

task_id="$(curl -fsS -X POST "$APP_URL/api/v1/tasks" \
  -H 'Content-Type: application/json' \
  -d '{"title":"Metrics demo","description":"Record CRUD metrics","status":"todo"}' | jq -er '.id')"
cleanup() { curl -fsS -X DELETE "$APP_URL/api/v1/tasks/$task_id" >/dev/null || true; }
trap cleanup EXIT

curl -fsS "$APP_URL/api/v1/tasks/$task_id"
curl -fsS -X PUT "$APP_URL/api/v1/tasks/$task_id" \
  -H 'Content-Type: application/json' \
  -d '{"title":"Metrics demo","description":"Finished","status":"done"}'
curl -fsS -X DELETE "$APP_URL/api/v1/tasks/$task_id"
trap - EXIT
echo 'All four successful task operations recorded. Allow about 45 seconds for scraping and query visibility.'
