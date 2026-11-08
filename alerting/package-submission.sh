#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
for name in 01-rules 02-firing 03-notifications-firing; do
  test -f "alerting/screenshots/$name.png" || { echo "Missing screenshot: $name.png" >&2; exit 1; }
done
output="$PWD/timepilot-alerting.zip"
test ! -e "$output" || { echo 'Archive already exists; move it aside before rebuilding.' >&2; exit 1; }
stage="$(mktemp -d /tmp/timepilot-alerting.XXXXXX)"
# Keep the staging folder on error for inspection; no recursive cleanup of user files.
cp -R alerting "$stage/alerting"
cp alerting/Отчёт.md alerting/README.md alerting/rules.yaml alerting/alertmanager.yaml alerting/stack.yaml alerting/kustomization.yaml "$stage/"
# Keep relative image/config links in the root report valid as well.
cp -R alerting/screenshots "$stage/screenshots"
cp -R cmd internal migrations k8s monitoring slo scripts "$stage/"
cp go.mod go.sum Dockerfile .dockerignore .env.example Makefile compose.yaml "$stage/"
(cd "$stage" && zip -qr "$output" .)
echo "Archive: $output"
echo "Temporary source copy: $stage"
