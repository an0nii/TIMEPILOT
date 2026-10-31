#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
# Warm up the prober before recording. Start this script when the graphs are visible.
restore() { ./slo/mode.sh healthy >/dev/null || true; }
trap restore EXIT
trap 'exit 130' INT TERM
./slo/mode.sh degraded
echo '50% injected errors for 90 seconds. Watch the 2-minute SLI graphs fall.'
remaining=90
while [ "$remaining" -gt 0 ]; do sleep 5; remaining=$((remaining - 5)); done
./slo/mode.sh healthy
echo 'Recovered. Waiting 150 seconds for the 2-minute window and query visibility.'
remaining=150
while [ "$remaining" -gt 0 ]; do sleep 5; remaining=$((remaining - 5)); done
echo 'Scenario finished; application is healthy.'
