#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
if [ "$#" -ne 1 ] || [ ! -f "$1" ]; then
  echo 'Usage: ./slo/package-submission.sh /absolute/path/to/screencast.mov' >&2
  exit 1
fi
video="$1"
output="$(pwd)/timepilot-slo.zip"
stage="$(mktemp -d)"
trap 'rm -rf "$stage"' EXIT
mkdir -p "$stage/timepilot-slo"
cp -R cmd internal migrations k8s scripts monitoring observability slo "$stage/timepilot-slo/"
cp Dockerfile .dockerignore .env.example go.mod go.sum Makefile compose.yaml README.md "$stage/timepilot-slo/"
cp slo/Отчёт.md "$stage/timepilot-slo/Отчёт.md"
cp "$video" "$stage/timepilot-slo/screencast.${video##*.}"
if command -v ditto >/dev/null 2>&1; then
  ditto -c -k --norsrc --keepParent "$stage/timepilot-slo" "$output"
else
  (cd "$stage" && zip -qr "$output" timepilot-slo)
fi
echo "Created $output"
