#!/bin/sh
set -eu

if [ "$#" -ne 1 ] || [ ! -f "$1" ]; then
  echo "Usage: $0 /path/to/screencast.mov" >&2
  exit 1
fi

VIDEO="$1"
OUTPUT="${OUTPUT:-timepilot-kubernetes.zip}"
STAGE="$(mktemp -d)"
ROOT="$(pwd)"
trap 'rm -rf "$STAGE"' EXIT

mkdir -p "$STAGE/timepilot/k8s" "$STAGE/timepilot/scripts"
cp Dockerfile .dockerignore .env.example Makefile compose.yaml README.md Отчёт.md СКРИНКАСТ.md go.mod go.sum "$STAGE/timepilot/"
cp -R cmd internal migrations "$STAGE/timepilot/"
cp k8s/*.yaml "$STAGE/timepilot/k8s/"
cp scripts/minikube-build.sh scripts/minikube-deploy.sh scripts/package-submission.sh "$STAGE/timepilot/scripts/"
cp "$VIDEO" "$STAGE/timepilot/screencast.${VIDEO##*.}"

if command -v ditto >/dev/null 2>&1; then
  ditto -c -k --norsrc --keepParent "$STAGE/timepilot" "$ROOT/$OUTPUT"
else
  (cd "$STAGE" && zip -qr "$ROOT/$OUTPUT" timepilot)
fi
echo "Created $OUTPUT"
