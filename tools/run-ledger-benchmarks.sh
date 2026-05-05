#!/bin/bash

# ==============================================================================
# Payloadless Trie Benchmark Suite
# ==============================================================================
#
# This script runs all ledger benchmarks sequentially and generates a report.
#
# Usage:
#   ./run-ledger-benchmarks.sh --checkpoint /path/to/root.checkpoint \
#                              --wal-dir /path/to/wal \
#                              --output-dir /path/to/results \
#                              --root-height 12345678
#
# Optional flags:
#   --phase N           Run only phase N (1-4)
#   --skip-import       Skip phase 1 (assumes storehouse already populated)
#   --workers "8,16,32,64"  Worker counts for phase 2 (default: 8,16,32,64)
#   --wal-counts "10,50,100,200,500"  WAL segment counts for phase 3
#
# ==============================================================================

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
WORKERS="8,16,32,64"
WAL_COUNTS="10,50,100,200,500"
PHASE=""
SKIP_IMPORT=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --checkpoint)
            CHECKPOINT_PATH="$2"
            shift 2
            ;;
        --wal-dir)
            WAL_DIR="$2"
            shift 2
            ;;
        --output-dir)
            OUTPUT_DIR="$2"
            shift 2
            ;;
        --root-height)
            ROOT_HEIGHT="$2"
            shift 2
            ;;
        --phase)
            PHASE="$2"
            shift 2
            ;;
        --skip-import)
            SKIP_IMPORT=true
            shift
            ;;
        --workers)
            WORKERS="$2"
            shift 2
            ;;
        --wal-counts)
            WAL_COUNTS="$2"
            shift 2
            ;;
        --help)
            echo "Usage: $0 --checkpoint PATH --wal-dir PATH --output-dir PATH --root-height HEIGHT"
            echo ""
            echo "Required:"
            echo "  --checkpoint PATH    Path to root checkpoint file"
            echo "  --wal-dir PATH       Path to WAL segment directory"
            echo "  --output-dir PATH    Path to output directory for results"
            echo "  --root-height HEIGHT Root height for the checkpoint"
            echo ""
            echo "Optional:"
            echo "  --phase N            Run only phase N (1-4)"
            echo "  --skip-import        Skip phase 1 (use existing storehouse)"
            echo "  --workers LIST       Worker counts for phase 2 (default: 8,16,32,64)"
            echo "  --wal-counts LIST    WAL segment counts for phase 3 (default: 10,50,100,200,500)"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Validate required arguments
if [[ -z "$CHECKPOINT_PATH" ]]; then
    echo -e "${RED}Error: --checkpoint is required${NC}"
    exit 1
fi

if [[ -z "$WAL_DIR" ]]; then
    echo -e "${RED}Error: --wal-dir is required${NC}"
    exit 1
fi

if [[ -z "$OUTPUT_DIR" ]]; then
    echo -e "${RED}Error: --output-dir is required${NC}"
    exit 1
fi

if [[ -z "$ROOT_HEIGHT" ]]; then
    echo -e "${RED}Error: --root-height is required${NC}"
    exit 1
fi

# Setup directories
STOREHOUSE_DIR="${OUTPUT_DIR}/storehouse"
RESULTS_DIR="${OUTPUT_DIR}/results"
LOGS_DIR="${OUTPUT_DIR}/logs"
MERGED_WAL_DIR="${OUTPUT_DIR}/merged-wal"

mkdir -p "$STOREHOUSE_DIR" "$RESULTS_DIR" "$LOGS_DIR" "$MERGED_WAL_DIR"

# Timestamp for this run
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
LOG_FILE="${LOGS_DIR}/benchmark-${TIMESTAMP}.log"

# Logging functions
log() {
    local msg="[$(date '+%Y-%m-%d %H:%M:%S')] $1"
    echo -e "$msg" | tee -a "$LOG_FILE"
}

log_phase() {
    echo "" | tee -a "$LOG_FILE"
    echo -e "${BLUE}========================================${NC}" | tee -a "$LOG_FILE"
    echo -e "${BLUE}$1${NC}" | tee -a "$LOG_FILE"
    echo -e "${BLUE}========================================${NC}" | tee -a "$LOG_FILE"
}

log_success() {
    echo -e "${GREEN}$1${NC}" | tee -a "$LOG_FILE"
}

log_error() {
    echo -e "${RED}$1${NC}" | tee -a "$LOG_FILE"
}

log_warn() {
    echo -e "${YELLOW}$1${NC}" | tee -a "$LOG_FILE"
}

# Find the util binary
UTIL_BIN="go run ./cmd/util"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FLOW_GO_DIR="$(dirname "$SCRIPT_DIR")"

cd "$FLOW_GO_DIR"

# Print configuration
log_phase "BENCHMARK CONFIGURATION"
log "Checkpoint:    $CHECKPOINT_PATH"
log "WAL Dir:       $WAL_DIR"
log "Output Dir:    $OUTPUT_DIR"
log "Root Height:   $ROOT_HEIGHT"
log "Workers:       $WORKERS"
log "WAL Counts:    $WAL_COUNTS"
log "Log File:      $LOG_FILE"

# Validate inputs
log_phase "VALIDATING INPUTS"

if [[ ! -f "$CHECKPOINT_PATH" ]]; then
    log_error "Checkpoint file not found: $CHECKPOINT_PATH"
    exit 1
fi
log_success "Checkpoint file exists"

if [[ ! -d "$WAL_DIR" ]]; then
    log_error "WAL directory not found: $WAL_DIR"
    exit 1
fi
log_success "WAL directory exists"

WAL_COUNT=$(ls -1 "$WAL_DIR" 2>/dev/null | wc -l)
log "WAL segments found: $WAL_COUNT"

# Check disk space
AVAILABLE_SPACE=$(df -BG "$OUTPUT_DIR" | tail -1 | awk '{print $4}' | sed 's/G//')
log "Available disk space: ${AVAILABLE_SPACE}G"

if [[ "$AVAILABLE_SPACE" -lt 100 ]]; then
    log_warn "Warning: Less than 100G available. Benchmarks may fail."
fi

# ==============================================================================
# PHASE 1: Import Checkpoint to Storehouse
# ==============================================================================
run_phase1() {
    log_phase "PHASE 1: Import Checkpoint to Storehouse"

    if [[ "$SKIP_IMPORT" == "true" ]]; then
        log_warn "Skipping Phase 1 (--skip-import specified)"
        return 0
    fi

    log "Input:  $CHECKPOINT_PATH"
    log "Output: $STOREHOUSE_DIR"

    RESULT_FILE="${RESULTS_DIR}/phase1_import.json"

    $UTIL_BIN ledger-benchmark import-checkpoint \
        --checkpoint "$CHECKPOINT_PATH" \
        --storehouse "$STOREHOUSE_DIR" \
        --output "$RESULT_FILE" \
        --root-height "$ROOT_HEIGHT" \
        --workers 4 \
        2>&1 | tee -a "$LOG_FILE"

    if [[ $? -eq 0 ]]; then
        log_success "Phase 1 complete. Results: $RESULT_FILE"
    else
        log_error "Phase 1 failed!"
        return 1
    fi
}

# ==============================================================================
# PHASE 2: Read Storehouse with Concurrent Workers
# ==============================================================================
run_phase2() {
    log_phase "PHASE 2: Read Storehouse (Concurrent Workers)"

    IFS=',' read -ra WORKER_ARRAY <<< "$WORKERS"

    for N in "${WORKER_ARRAY[@]}"; do
        log ""
        log "--- Running with N=$N workers ---"

        RESULT_FILE="${RESULTS_DIR}/phase2_read_N${N}.json"

        $UTIL_BIN ledger-benchmark read-storehouse \
            --checkpoint "$CHECKPOINT_PATH" \
            --storehouse "$STOREHOUSE_DIR" \
            --output "$RESULT_FILE" \
            --height "$ROOT_HEIGHT" \
            --workers "$N" \
            2>&1 | tee -a "$LOG_FILE"

        if [[ $? -eq 0 ]]; then
            log_success "N=$N complete. Results: $RESULT_FILE"
        else
            log_error "N=$N failed!"
        fi
    done
}

# ==============================================================================
# PHASE 3: Merge WAL Files
# ==============================================================================
run_phase3() {
    log_phase "PHASE 3: Merge WAL Files"

    IFS=',' read -ra COUNT_ARRAY <<< "$WAL_COUNTS"

    for N in "${COUNT_ARRAY[@]}"; do
        log ""
        log "--- Merging N=$N WAL segments ---"

        RESULT_FILE="${RESULTS_DIR}/phase3_merge_N${N}.json"

        $UTIL_BIN ledger-benchmark merge-wal \
            --wal-dir "$WAL_DIR" \
            --output-dir "$MERGED_WAL_DIR" \
            --results "$RESULT_FILE" \
            --segments "$N" \
            2>&1 | tee -a "$LOG_FILE"

        if [[ $? -eq 0 ]]; then
            log_success "N=$N complete. Results: $RESULT_FILE"
        else
            log_error "N=$N failed!"
        fi
    done
}

# ==============================================================================
# PHASE 4: Apply WAL to Trie
# ==============================================================================
run_phase4() {
    log_phase "PHASE 4: Apply WAL to Trie"

    IFS=',' read -ra COUNT_ARRAY <<< "$WAL_COUNTS"

    for N in "${COUNT_ARRAY[@]}"; do
        MERGED_WAL="${MERGED_WAL_DIR}/merged-${N}.wal"

        if [[ ! -f "$MERGED_WAL" ]]; then
            log_warn "Merged WAL not found for N=$N: $MERGED_WAL (skipping)"
            continue
        fi

        log ""
        log "--- Applying merged WAL (N=$N) ---"

        RESULT_FILE="${RESULTS_DIR}/phase4_apply_N${N}.json"

        $UTIL_BIN ledger-benchmark apply-wal \
            --checkpoint "$CHECKPOINT_PATH" \
            --merged-wal "$MERGED_WAL" \
            --output "$RESULT_FILE" \
            2>&1 | tee -a "$LOG_FILE"

        if [[ $? -eq 0 ]]; then
            log_success "N=$N complete. Results: $RESULT_FILE"
        else
            log_error "N=$N failed!"
        fi
    done
}

# ==============================================================================
# GENERATE REPORT
# ==============================================================================
generate_report() {
    log_phase "GENERATING REPORT"

    REPORT_FILE="${RESULTS_DIR}/benchmark_report_${TIMESTAMP}.md"

    cat > "$REPORT_FILE" << EOF
# Payloadless Trie Benchmark Report

Generated: $(date '+%Y-%m-%d %H:%M:%S')
Checkpoint: $CHECKPOINT_PATH
Root Height: $ROOT_HEIGHT

## Results Summary

### Phase 1: Checkpoint Import
EOF

    if [[ -f "${RESULTS_DIR}/phase1_import.json" ]]; then
        cat >> "$REPORT_FILE" << EOF
\`\`\`json
$(cat "${RESULTS_DIR}/phase1_import.json")
\`\`\`
EOF
    else
        echo "No results (skipped or failed)" >> "$REPORT_FILE"
    fi

    cat >> "$REPORT_FILE" << EOF

### Phase 2: Storehouse Read Performance

| Workers | Total Time (s) | Registers | Rate (reg/s) | Avg Latency (us) | P99 Latency (us) |
|---------|----------------|-----------|--------------|------------------|------------------|
EOF

    IFS=',' read -ra WORKER_ARRAY <<< "$WORKERS"
    for N in "${WORKER_ARRAY[@]}"; do
        RESULT_FILE="${RESULTS_DIR}/phase2_read_N${N}.json"
        if [[ -f "$RESULT_FILE" ]]; then
            TIME=$(jq -r '.results.total_time_sec' "$RESULT_FILE")
            REGS=$(jq -r '.results.register_count' "$RESULT_FILE")
            RATE=$(jq -r '.results.rate_registers_per_sec' "$RESULT_FILE")
            AVG=$(jq -r '.results.avg_latency_us' "$RESULT_FILE")
            P99=$(jq -r '.results.p99_latency_us' "$RESULT_FILE")
            echo "| $N | $TIME | $REGS | $RATE | $AVG | $P99 |" >> "$REPORT_FILE"
        fi
    done

    cat >> "$REPORT_FILE" << EOF

### Phase 3: WAL Merge Performance

| Segments | Merge Time (s) | Input Size | Output Size | Pre-merge | Post-merge | Dedup % |
|----------|----------------|------------|-------------|-----------|------------|---------|
EOF

    IFS=',' read -ra COUNT_ARRAY <<< "$WAL_COUNTS"
    for N in "${COUNT_ARRAY[@]}"; do
        RESULT_FILE="${RESULTS_DIR}/phase3_merge_N${N}.json"
        if [[ -f "$RESULT_FILE" ]]; then
            TIME=$(jq -r '.results.merge_time_sec' "$RESULT_FILE")
            INPUT=$(jq -r '.results.input_size_bytes' "$RESULT_FILE")
            OUTPUT=$(jq -r '.results.output_size_bytes' "$RESULT_FILE")
            PRE=$(jq -r '.results.total_updates_pre_merge' "$RESULT_FILE")
            POST=$(jq -r '.results.unique_registers_post_merge' "$RESULT_FILE")
            DEDUP=$(jq -r '.results.deduplication_ratio_pct' "$RESULT_FILE")
            echo "| $N | $TIME | $INPUT | $OUTPUT | $PRE | $POST | $DEDUP |" >> "$REPORT_FILE"
        fi
    done

    cat >> "$REPORT_FILE" << EOF

### Phase 4: WAL Apply Performance

| Source | Apply Time (s) | Registers | Rate (reg/s) | Peak Memory (MB) |
|--------|----------------|-----------|--------------|------------------|
EOF

    for N in "${COUNT_ARRAY[@]}"; do
        RESULT_FILE="${RESULTS_DIR}/phase4_apply_N${N}.json"
        if [[ -f "$RESULT_FILE" ]]; then
            TIME=$(jq -r '.results.apply_time_sec' "$RESULT_FILE")
            REGS=$(jq -r '.results.registers_updated' "$RESULT_FILE")
            RATE=$(jq -r '.results.apply_rate_per_sec' "$RESULT_FILE")
            MEM=$(jq -r '.resources.peak_memory_mb' "$RESULT_FILE")
            echo "| N=$N | $TIME | $REGS | $RATE | $MEM |" >> "$REPORT_FILE"
        fi
    done

    cat >> "$REPORT_FILE" << EOF

## Raw Results

All JSON result files are available in: \`$RESULTS_DIR\`

## Log File

Full execution log: \`$LOG_FILE\`
EOF

    log_success "Report generated: $REPORT_FILE"
}

# ==============================================================================
# MAIN EXECUTION
# ==============================================================================

START_TIME=$(date +%s)

if [[ -z "$PHASE" ]]; then
    # Run all phases
    run_phase1
    run_phase2
    run_phase3
    run_phase4
else
    # Run specific phase
    case $PHASE in
        1) run_phase1 ;;
        2) run_phase2 ;;
        3) run_phase3 ;;
        4) run_phase4 ;;
        *) log_error "Invalid phase: $PHASE"; exit 1 ;;
    esac
fi

generate_report

END_TIME=$(date +%s)
TOTAL_TIME=$((END_TIME - START_TIME))

log_phase "BENCHMARK COMPLETE"
log "Total execution time: ${TOTAL_TIME} seconds ($(($TOTAL_TIME / 60)) minutes)"
log "Results directory: $RESULTS_DIR"
log "Report: ${RESULTS_DIR}/benchmark_report_${TIMESTAMP}.md"
log "Log file: $LOG_FILE"
