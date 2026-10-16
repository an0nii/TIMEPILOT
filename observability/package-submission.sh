#!/bin/sh
set -eu

SCREENSHOTS="observability/screenshots"
for screenshot in 01-raw-logs.png 02-field-filter.png 03-stats.png 04-top.png; do
  if [ ! -f "${SCREENSHOTS}/${screenshot}" ]; then
    echo "Missing ${SCREENSHOTS}/${screenshot}" >&2
    exit 1
  fi
done

OUTPUT="${OUTPUT:-timepilot-victorialogs.zip}"
STAGE="$(mktemp -d)"
ROOT="$(pwd)"
trap 'rm -rf "$STAGE"' EXIT

mkdir -p "$STAGE/timepilot-victorialogs/observability"
cp Dockerfile Makefile README.md go.mod go.sum "$STAGE/timepilot-victorialogs/"
cp -R cmd internal migrations k8s "$STAGE/timepilot-victorialogs/"
cp -R observability/*.yaml observability/*.sh observability/README.md \
  "$STAGE/timepilot-victorialogs/observability/"
cp observability/Отчёт.md "$STAGE/timepilot-victorialogs/Отчёт.md"
cp -R observability/screenshots "$STAGE/timepilot-victorialogs/screenshots"

if command -v ditto >/dev/null 2>&1; then
  ditto -c -k --norsrc --keepParent "$STAGE/timepilot-victorialogs" "$ROOT/$OUTPUT"
else
  (cd "$STAGE" && zip -qr "$ROOT/$OUTPUT" timepilot-victorialogs)
fi
echo "Created $OUTPUT"
