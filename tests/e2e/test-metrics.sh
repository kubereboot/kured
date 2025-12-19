#!/usr/bin/env bash

expected="$1"
if [[ "$expected" != "0" && "$expected" != "1" ]]; then
    echo "You should give an argument to this script, the gauge value (0 or 1)"
    exit 1
fi

HOST="${HOST:-localhost}"
PORT="${PORT:-30000}"
NODENAME="${NODENAME-chart-testing-control-plane}"

reboot_required=$(docker exec "$NODENAME" curl "http://$HOST:$PORT/metrics" | awk '/^kured_reboot_required/{print $2}')
if [[ "$reboot_required" == "$expected" ]]; then
    echo "Test success"
else
    echo "Test failed"
    exit 1
fi