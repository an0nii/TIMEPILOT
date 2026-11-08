#!/bin/sh
set -eu
kubectl get pods,svc -n monitoring
curl -fsS http://127.0.0.1:8880/api/v1/rules
printf '\n'
curl -fsS http://127.0.0.1:9093/-/ready
printf '\n'
curl -fsS http://127.0.0.1:9095/api/notifications
printf '\n'
