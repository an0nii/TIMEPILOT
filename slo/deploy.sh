#!/bin/sh
set -eu
cd "$(dirname "$0")/.."

kubectl get deployment timepilot -n timepilot >/dev/null
kubectl apply -f k8s/01-config.yaml -f k8s/04-app.yaml
kubectl set env deployment/timepilot -n timepilot SLO_DEMO_ENABLED=true
kubectl rollout restart deployment/timepilot -n timepilot
kubectl rollout status deployment/timepilot -n timepilot --timeout=180s

helm repo add vm https://victoriametrics.github.io/helm-charts/ --force-update
helm repo update vm
helm upgrade --install vm-single vm/victoria-metrics-single \
  --version 0.29.0 --namespace monitoring --create-namespace \
  -f monitoring/victoria-metrics-values.yaml -f slo/victoria-metrics-values.yaml \
  --wait --timeout 5m

kubectl apply -f slo/01-prober.yaml
kubectl rollout restart deployment/timepilot-prober -n timepilot
kubectl rollout status deployment/timepilot-prober -n timepilot --timeout=180s
kubectl get pods -n timepilot
echo 'Demo controls enabled on the internal metrics port. Mode starts healthy.'
