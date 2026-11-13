#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
kubectl get statefulset victoria-metrics -n monitoring >/dev/null
kubectl get deployment timepilot -n timepilot >/dev/null
python3 grafana/build_dashboards.py
kubectl create configmap timepilot-grafana-dashboard -n monitoring \
  --from-file=grafana/dashboards/timepilot.json --dry-run=client -o yaml | kubectl apply -f -
helm repo add grafana-community https://grafana-community.github.io/helm-charts --force-update
helm repo update grafana-community
helm upgrade --install grafana grafana-community/grafana --version 14.4.1 \
  --namespace monitoring --values grafana/values.yaml --wait --timeout 5m
echo 'Run: kubectl port-forward -n monitoring svc/grafana 3000:80'
echo 'Then: python3 grafana/api.py import-postgres && python3 grafana/api.py export'
