#!/bin/sh
set -eu

if ! kubectl get deployment timepilot --namespace timepilot >/dev/null 2>&1; then
  echo "TimePilot is not deployed. Run ./scripts/minikube-deploy.sh first." >&2
  exit 1
fi

kubectl apply -f observability/00-namespace.yaml
kubectl apply -f observability/01-victoria-logs.yaml
kubectl rollout status deployment/victoria-logs --namespace observability --timeout=300s

kubectl apply -f observability/03-vector.yaml
kubectl rollout status daemonset/vector --namespace observability --timeout=300s

kubectl get pods,services,pvc --namespace observability --output=wide
