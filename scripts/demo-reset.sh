#!/bin/bash
# Demo Reset Script
# Resets demo data through the application API.
# Run via cron: 0 * * * * /app/scripts/demo-reset.sh

set -euo pipefail

echo "$(date): Starting demo reset..."

# Check if DEMO_MODE is enabled
if [ "${DEMO_MODE:-}" != "true" ]; then
    echo "DEMO_MODE is not enabled. Skipping reset."
    exit 0
fi

if [ -z "${DEMO_RESET_SECRET:-}" ]; then
    echo "ERROR: DEMO_RESET_SECRET is not set"
    exit 1
fi

reset_url="${DEMO_RESET_URL:-}"
if [ -z "$reset_url" ] && [ -n "${API_BASE_URL:-}" ]; then
    reset_url="${API_BASE_URL%/}/api/demo/reset"
fi

if [ -z "$reset_url" ]; then
    echo "ERROR: DEMO_RESET_URL or API_BASE_URL is not set"
    exit 1
fi

if [ -n "${DEMO_RESET_USER:-}" ]; then
    case "$DEMO_RESET_USER" in
        1|2|3|4) ;;
        *)
            echo "ERROR: DEMO_RESET_USER must be 1, 2, 3, or 4"
            exit 1
            ;;
    esac

    separator="?"
    if [[ "$reset_url" == *"?"* ]]; then
        separator="&"
    fi
    reset_url="${reset_url}${separator}user=${DEMO_RESET_USER}"
fi

response_file="${TMPDIR:-/tmp}/open-accounting-demo-reset-response.$$"
trap 'rm -f "$response_file"' EXIT

echo "Calling demo reset endpoint..."
http_code=$(curl -sS -o "$response_file" -w "%{http_code}" \
    -X POST "$reset_url" \
    -H "X-Demo-Secret: ${DEMO_RESET_SECRET}") || {
    echo "ERROR: Demo reset request failed"
    if [ -s "$response_file" ]; then
        cat "$response_file"
    fi
    exit 1
}

if [[ ! "$http_code" =~ ^2 ]]; then
    echo "ERROR: Demo reset endpoint returned HTTP $http_code"
    if [ -s "$response_file" ]; then
        cat "$response_file"
    fi
    exit 1
fi

echo "$(date): Demo reset completed successfully!"
