#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
kubectl get statefulset victoria-metrics -n monitoring >/dev/null
kubectl get deployment timepilot -n timepilot >/dev/null
kubectl set env deployment/timepilot -n timepilot SLO_DEMO_ENABLED=true
kubectl rollout restart deployment/timepilot -n timepilot
kubectl rollout status deployment/timepilot -n timepilot --timeout=180s
kubectl apply -f slo/01-prober.yaml
kubectl rollout status deployment/timepilot-prober -n timepilot --timeout=180s
kubectl apply -k alerting/
kubectl rollout restart deployment/alert-inbox -n monitoring
for deployment in vmalert alertmanager alert-inbox; do
  kubectl rollout status "deployment/$deployment" -n monitoring --timeout=180s
done
echo 'Forward ports using the commands in alerting/README.md.'
