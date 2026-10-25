#!/bin/sh
set -eu
cd "$(dirname "$0")/.."

command -v helm >/dev/null 2>&1 || { echo 'Install Helm first: brew install helm' >&2; exit 1; }
kubectl get deployment timepilot -n timepilot >/dev/null

kubectl apply -f k8s/01-config.yaml -f k8s/02-postgres.yaml -f k8s/04-app.yaml
# The local image tag stays timepilot:k8s when rebuilding.
kubectl rollout restart deployment/timepilot -n timepilot
kubectl rollout status statefulset/postgres -n timepilot --timeout=300s
kubectl rollout status deployment/timepilot -n timepilot --timeout=300s

helm repo add vm https://victoriametrics.github.io/helm-charts/ --force-update
helm repo update vm
helm upgrade --install vm-single vm/victoria-metrics-single \
  --version 0.29.0 --namespace monitoring --create-namespace \
  --values monitoring/victoria-metrics-values.yaml --wait --timeout 5m
kubectl get pods,services,pvc -n monitoring
