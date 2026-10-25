#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
if [ "$#" -ne 1 ] || [ ! -f "$1" ]; then
  echo 'Usage: ./monitoring/package-submission.sh /absolute/path/to/screencast.mov' >&2
  exit 1
fi
video="$1"
output="$(pwd)/timepilot-metrics.zip"
stage="$(mktemp -d)"
trap 'rm -rf "$stage"' EXIT
mkdir -p "$stage/timepilot-metrics"
cp -R cmd internal migrations k8s scripts monitoring observability "$stage/timepilot-metrics/"
cp Dockerfile .dockerignore .env.example go.mod go.sum Makefile compose.yaml README.md "$stage/timepilot-metrics/"
cp monitoring/Отчёт.md "$stage/timepilot-metrics/Отчёт.md"
cp "$video" "$stage/timepilot-metrics/screencast.${video##*.}"
if command -v ditto >/dev/null 2>&1; then
  ditto -c -k --norsrc --keepParent "$stage/timepilot-metrics" "$output"
else
  (cd "$stage" && zip -qr "$output" timepilot-metrics)
fi
echo "Created $output"
