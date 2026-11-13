#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
video="${1:-}"
test -n "$video" && test -f "$video" || { echo 'Usage: ./grafana/package-submission.sh /absolute/path/to/screencast.mov' >&2; exit 1; }
case "$video" in *.mov|*.mp4) ;; *) echo 'Expected .mov or .mp4 screencast' >&2; exit 1 ;; esac
for name in 01-app-overview 02-http-red 03-go-runtime 04-postgres-overview 05-postgres-details; do
  test -f "grafana/screenshots/$name.png" || { echo "Missing screenshot: $name" >&2; exit 1; }
done
test -f grafana/exports/timepilot.json || { echo 'Export dashboards first: python3 grafana/api.py export' >&2; exit 1; }
archive="$PWD/timepilot-grafana.zip"
test ! -e "$archive" || { echo 'Archive already exists; move it aside before rebuilding.' >&2; exit 1; }
stage="$(mktemp -d /tmp/timepilot-grafana.XXXXXX)"
cp -R grafana "$stage/grafana"
cp grafana/Отчёт.md "$stage/Отчёт.md"
# Keep root report relative links functional.
cp -R grafana/screenshots grafana/exports grafana/vendor "$stage/"
cp grafana/values.yaml grafana/build_dashboards.py grafana/README.md "$stage/"
cp -R cmd internal migrations k8s monitoring slo scripts "$stage/"
cp go.mod go.sum Dockerfile .dockerignore .env.example "$stage/"
cp "$video" "$stage/screencast.${video##*.}"
(cd "$stage" && zip -qr "$archive" .)
echo "Archive: $archive"
echo "Temporary source copy: $stage"
