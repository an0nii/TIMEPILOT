#!/bin/sh
set -eu

APP_PORT="${APP_PORT:-18080}"
LOGS_PORT="${LOGS_PORT:-19428}"

kubectl port-forward --namespace timepilot service/timepilot "${APP_PORT}:8080" >/tmp/timepilot-app-port-forward.log 2>&1 &
APP_FORWARD_PID=$!
kubectl port-forward --namespace observability service/victoria-logs "${LOGS_PORT}:9428" >/tmp/timepilot-vlogs-port-forward.log 2>&1 &
LOGS_FORWARD_PID=$!

cleanup() {
  kill "${APP_FORWARD_PID}" "${LOGS_FORWARD_PID}" 2>/dev/null || true
  wait "${APP_FORWARD_PID}" "${LOGS_FORWARD_PID}" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

sleep 3
curl --fail --silent "http://127.0.0.1:${APP_PORT}/healthz" >/dev/null

attempt=1
while [ "${attempt}" -le 15 ]; do
  result="$(curl --fail --silent \
    --data-urlencode 'query=_time:30m "service.name":="timepilot"' \
    --data-urlencode 'limit=3' \
    "http://127.0.0.1:${LOGS_PORT}/select/logsql/query")"
  if [ -n "${result}" ]; then
    printf '%s\n' "${result}"
    echo "VictoriaLogs contains TimePilot logs."
    exit 0
  fi
  attempt=$((attempt + 1))
  sleep 2
done

echo "No TimePilot logs found in VictoriaLogs." >&2
exit 1
