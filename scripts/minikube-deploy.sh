#!/bin/sh
set -eu

kubectl apply -f k8s/00-namespace.yaml
kubectl apply -f k8s/
kubectl wait --namespace timepilot --for=condition=ready pod/postgres-0 --timeout=180s
kubectl wait --namespace timepilot --for=condition=complete job/timepilot-migrate --timeout=180s
kubectl rollout status deployment/timepilot --namespace timepilot --timeout=180s
kubectl get pods --namespace timepilot --output=wide
