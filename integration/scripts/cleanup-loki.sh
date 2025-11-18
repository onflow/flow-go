#!/bin/bash
#
# Script to clean up old logs from Loki
#
# Usage:
#   ./scripts/cleanup-loki.sh --all                    # Delete all Loki logs
#   ./scripts/cleanup-loki.sh --older-than 7d          # Delete logs older than 7 days
#   ./scripts/cleanup-loki.sh --test TestName          # Delete logs from specific test
#   ./scripts/cleanup-loki.sh --job go-test            # Delete logs from specific job
#   ./scripts/cleanup-loki.sh --selector '{job="go-test"}' --older-than 1d
#

set -e

LOKI_URL="${LOKI_URL:-http://localhost:3100}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCALNET_DIR="$SCRIPT_DIR/../localnet"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Options:
    --all                    Delete all Loki data (stops Loki, deletes data directory, restarts)
    --older-than DURATION    Delete logs older than DURATION (e.g., 7d, 24h, 1w)
    --test TEST_NAME         Delete logs from specific test name
    --job JOB_NAME           Delete logs from specific job (default: go-test)
    --selector SELECTOR      Custom label selector (e.g., '{job="go-test", test="TestName"}')
    --dry-run                Show what would be deleted without actually deleting
    --help                   Show this help message

Examples:
    $0 --all
    $0 --older-than 7d
    $0 --test TestVerifyScheduledTransactions
    $0 --job go-test --older-than 24h
    $0 --selector '{job="go-test"}' --older-than 1d
EOF
}

check_loki() {
    if ! curl -s "$LOKI_URL/ready" > /dev/null 2>&1; then
        echo -e "${RED}Error: Loki is not accessible at $LOKI_URL${NC}" >&2
        echo "Make sure Loki is running:" >&2
        echo "  cd integration/localnet && docker compose -f docker-compose.metrics.yml up -d loki" >&2
        exit 1
    fi
}

delete_all() {
    echo -e "${YELLOW}Warning: This will delete ALL Loki logs!${NC}"
    read -p "Are you sure? (yes/no): " confirm
    if [ "$confirm" != "yes" ]; then
        echo "Cancelled."
        exit 0
    fi

    echo "Stopping Loki..."
    cd "$LOCALNET_DIR"
    docker compose -f docker-compose.metrics.yml stop loki || true

    echo "Deleting Loki data directory..."
    rm -rf ./data/loki

    echo "Restarting Loki..."
    docker compose -f docker-compose.metrics.yml up -d loki

    echo -e "${GREEN}All Loki logs have been deleted.${NC}"
}

parse_duration() {
    local duration=$1
    local amount=${duration%[a-z]}
    local unit=${duration#$amount}

    case $unit in
        s|sec|second|seconds)
            echo $((amount * 1))
            ;;
        m|min|minute|minutes)
            echo $((amount * 60))
            ;;
        h|hour|hours)
            echo $((amount * 3600))
            ;;
        d|day|days)
            echo $((amount * 86400))
            ;;
        w|week|weeks)
            echo $((amount * 604800))
            ;;
        *)
            echo "Invalid duration unit: $unit" >&2
            echo "Valid units: s, m, h, d, w" >&2
            exit 1
            ;;
    esac
}

delete_via_api() {
    local selector=$1
    local start_time=$2
    local end_time=$3
    local dry_run=${4:-false}

    if [ "$dry_run" = "true" ]; then
        echo -e "${YELLOW}[DRY RUN] Would delete logs:${NC}"
        echo "  Selector: $selector"
        echo "  Start: $(date -u -d "@$((start_time / 1000000000))" 2>/dev/null || echo "epoch $start_time")"
        echo "  End: $(date -u -d "@$((end_time / 1000000000))" 2>/dev/null || echo "epoch $end_time")"
        return 0
    fi

    local response=$(curl -s -w "\n%{http_code}" -X POST "$LOKI_URL/loki/api/v1/delete" \
        -H 'Content-Type: application/json' \
        -d "{
            \"selector\": \"$selector\",
            \"start\": \"$start_time\",
            \"end\": \"$end_time\"
        }")

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | sed '$d')

    if [ "$http_code" = "204" ] || [ "$http_code" = "200" ]; then
        echo -e "${GREEN}Successfully submitted deletion request.${NC}"
        echo "Note: Deletion is processed asynchronously by the compactor."
    else
        echo -e "${RED}Error: Deletion request failed (HTTP $http_code)${NC}" >&2
        echo "$body" >&2
        exit 1
    fi
}

# Parse arguments
DELETE_ALL=false
OLDER_THAN=""
TEST_NAME=""
JOB_NAME="go-test"
SELECTOR=""
DRY_RUN=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --all)
            DELETE_ALL=true
            shift
            ;;
        --older-than)
            OLDER_THAN="$2"
            shift 2
            ;;
        --test)
            TEST_NAME="$2"
            shift 2
            ;;
        --job)
            JOB_NAME="$2"
            shift 2
            ;;
        --selector)
            SELECTOR="$2"
            shift 2
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown option: $1" >&2
            usage
            exit 1
            ;;
    esac
done

# Handle --all flag
if [ "$DELETE_ALL" = "true" ]; then
    delete_all
    exit 0
fi

# Check if Loki is accessible (unless dry-run and we're deleting all)
if [ "$DRY_RUN" = "false" ]; then
    check_loki
fi

# Build selector
if [ -n "$SELECTOR" ]; then
    # Use custom selector as-is
    selector="$SELECTOR"
elif [ -n "$TEST_NAME" ]; then
    selector="{job=\"$JOB_NAME\", test=\"$TEST_NAME\"}"
elif [ -n "$JOB_NAME" ]; then
    selector="{job=\"$JOB_NAME\"}"
else
    echo "Error: Must specify --selector, --test, or --job" >&2
    usage
    exit 1
fi

# Calculate time range
if [ -n "$OLDER_THAN" ]; then
    # Delete logs older than specified duration
    seconds=$(parse_duration "$OLDER_THAN")
    end_time=$(($(date +%s) - seconds))
    start_time=0
    end_time_ns="${end_time}000000000"
    start_time_ns="0"
else
    # Delete all logs matching selector
    start_time_ns="0"
    end_time_ns="$(date +%s)000000000"
fi

# Perform deletion
delete_via_api "$selector" "$start_time_ns" "$end_time_ns" "$DRY_RUN"

if [ "$DRY_RUN" = "false" ]; then
    echo ""
    echo "To check disk usage:"
    echo "  du -sh $LOCALNET_DIR/data/loki"
fi

