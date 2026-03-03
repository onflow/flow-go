#!/bin/bash
#
# Script to pipe test output to Loki for viewing in Grafana
#
# Usage:
#   cd integration
#   go test -failfast ./tests/verification/ --run=TestVerifyScheduledTransactions -v | ./scripts/send-to-loki.sh
#
# Or with custom labels:
#   go test -failfast ./tests/verification/ --run=TestVerifyScheduledTransactions -v | ./scripts/send-to-loki.sh -test=TestVerifyScheduledTransactions -file=verification_test.go
#
# Make sure Loki is running (e.g., via docker-compose in integration/localnet)

set -e

# Default values
LOKI_URL="${LOKI_URL:-http://localhost:3100/loki/api/v1/push}"
JOB="${JOB:-go-test}"
TEST_NAME=""
TEST_FILE=""
BATCH_SIZE="${BATCH_SIZE:-100}"
BATCH_DELAY="${BATCH_DELAY:-2s}"

# Parse arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    -url|--url)
      LOKI_URL="$2"
      shift 2
      ;;
    -job|--job)
      JOB="$2"
      shift 2
      ;;
    -test|--test)
      TEST_NAME="$2"
      shift 2
      ;;
    -file|--file)
      TEST_FILE="$2"
      shift 2
      ;;
    -batch-size|--batch-size)
      BATCH_SIZE="$2"
      shift 2
      ;;
    -batch-delay|--batch-delay)
      BATCH_DELAY="$2"
      shift 2
      ;;
    *)
      echo "Unknown option: $1"
      echo "Usage: $0 [-url URL] [-job JOB] [-test TEST_NAME] [-file TEST_FILE] [-batch-size SIZE] [-batch-delay DELAY]"
      exit 1
      ;;
  esac
done

# Check if Go tool exists
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GO_TOOL="$SCRIPT_DIR/send-to-loki.go"

if ! command -v go &> /dev/null; then
  echo "Error: Go is required to run this script. Please install Go or use the pre-built binary." >&2
  exit 1
fi

if [ ! -f "$GO_TOOL" ]; then
  echo "Error: send-to-loki.go not found at $GO_TOOL" >&2
  exit 1
fi

# Use Go tool to send logs to Loki
go run "$GO_TOOL" \
  -url="$LOKI_URL" \
  -job="$JOB" \
  -test="$TEST_NAME" \
  -file="$TEST_FILE" \
  -batch-size="$BATCH_SIZE" \
  -batch-delay="$BATCH_DELAY"

