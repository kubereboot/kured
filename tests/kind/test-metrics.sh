#!/usr/bin/env bash

expected="$1"
if [[ "$expected" != "0" && "$expected" != "1" ]]; then
    echo "You should give an argument to this script, the gauge value (0 or 1)"
    exit 1
fi

HOST="${HOST:-localhost}"
PORT="${PORT:-30000}"
NODENAME="${NODENAME-chart-testing-control-plane}"

# --- Add retry logic for robustness in CI environments ---
# This makes the test more resilient to timing issues where the kured service
# might not be immediately available after the kind cluster starts.
MAX_RETRIES=12
RETRY_DELAY=5
CURL_TIMEOUT=10

echo "Polling for 'kured_reboot_required' metric to be '$expected'..."
echo "Retrying up to $MAX_RETRIES times, with a $RETRY_DELAY second delay and a ${CURL_TIMEOUT}s timeout per attempt."

for i in $(seq 1 "$MAX_RETRIES"); do
    # --fail: exit non-zero on server error (4xx, 5xx)
    # --silent: hide progress bar but still show errors on stderr
    # --max-time: prevent hanging
    metrics_output=$(docker exec "$NODENAME" curl --fail --silent --max-time "$CURL_TIMEOUT" "http://$HOST:$PORT/metrics")
    curl_exit_code=$?

    if [[ $curl_exit_code -eq 0 ]]; then
        reboot_required=$(echo "$metrics_output" | awk '/^kured_reboot_required/{print $2}')

        if [[ -z "$reboot_required" ]]; then
            echo "Attempt $i/$MAX_RETRIES: Curl succeeded, but 'kured_reboot_required' metric not found in output."
        elif [[ "$reboot_required" == "$expected" ]]; then
            echo "Test success: Found 'kured_reboot_required: $reboot_required' on attempt $i."
            exit 0
        else
            echo "Attempt $i/$MAX_RETRIES: Metric 'kured_reboot_required' is '$reboot_required', but expected '$expected'."
        fi
    else
        echo "Attempt $i/$MAX_RETRIES: Failed to curl metrics endpoint (exit code: $curl_exit_code)."
    fi

    if [[ $i -lt $MAX_RETRIES ]]; then
        sleep "$RETRY_DELAY"
    fi
done

echo "Test failed: Metric 'kured_reboot_required' did not reach expected state '$expected' after $MAX_RETRIES attempts."
exit 1