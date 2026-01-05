#!/bin/bash
# Collect stable benchmark data for a single Go version

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Configuration
WARMUP_COUNT=3
BENCH_COUNT=20
BENCH_TIME="3s"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

usage() {
    echo "Usage: $0 <go-version> [options]"
    echo
    echo "Arguments:"
    echo "  go-version    Go version to benchmark (e.g., 1.24, 1.25)"
    echo
    echo "Options:"
    echo "  --skip-warmup     Skip warm-up iterations"
    echo "  --skip-checks     Skip system stability checks"
    echo "  --count N         Number of benchmark runs (default: ${BENCH_COUNT})"
    echo
    echo "Example:"
    echo "  $0 1.24"
    echo "  $0 1.25 --count 30"
}

if [ $# -lt 1 ]; then
    usage
    exit 1
fi

GO_VERSION=$1
shift

# Parse options
SKIP_WARMUP=false
SKIP_CHECKS=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --skip-warmup)
            SKIP_WARMUP=true
            shift
            ;;
        --skip-checks)
            SKIP_CHECKS=true
            shift
            ;;
        --count)
            BENCH_COUNT=$2
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

# Find Go binary
GO_BIN=$("${SCRIPT_DIR}/setup-go-versions.sh" path "$GO_VERSION" 2>/dev/null)

if [ -z "$GO_BIN" ] || [ ! -x "$GO_BIN" ]; then
    echo "Error: Go $GO_VERSION not found"
    echo "Install it first with: ./tools/setup-go-versions.sh install <full-version>"
    exit 1
fi

# Check benchstat is installed
if ! command -v benchstat &> /dev/null; then
    echo "Warning: benchstat not installed (needed for comparisons)"
    echo "Install with: ./tools/install-tools.sh"
fi

TIMESTAMP=$(date +"%Y-%m-%d_%H-%M-%S")
OUTPUT_DIR="${SCRIPT_DIR}/../results/stable/go${GO_VERSION}"
OUTPUT_FILE="${OUTPUT_DIR}/${TIMESTAMP}.txt"

mkdir -p "$OUTPUT_DIR"

echo "=== Stable Benchmark Collection ==="
echo
echo "Version:  Go $GO_VERSION ($GO_BIN)"
echo "Output:   $OUTPUT_FILE"
echo "Runs:     ${BENCH_COUNT} iterations × ${BENCH_TIME} each"
echo

# Step 1: System check (optional)
if [ "$SKIP_CHECKS" = false ]; then
    echo "Step 1: System stability check"
    if "${SCRIPT_DIR}/system-check.sh"; then
        echo -e "${GREEN}✓ System checks passed${NC}"
    else
        echo -e "${YELLOW}⚠ System checks have warnings${NC}"
        read -p "Continue anyway? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
    fi
    echo
fi

# Step 2: Warm-up (optional)
if [ "$SKIP_WARMUP" = false ]; then
    echo "Step 2: Warm-up (${WARMUP_COUNT} iterations)"
    cd "${SCRIPT_DIR}/../benchmarks"

    echo "  → Warming up CPU caches and branch predictors..."
    GOTOOLCHAIN=local "$GO_BIN" test -bench=. -benchmem -count=$WARMUP_COUNT -benchtime=1s ./core/ > /dev/null 2>&1

    echo -e "${GREEN}✓ System warmed up${NC}"
    echo

    cd "${SCRIPT_DIR}/.."
fi

# Step 3: Collect benchmarks
STEP_NUM=3
if [ "$SKIP_CHECKS" = true ]; then
    STEP_NUM=$((STEP_NUM - 1))
fi
if [ "$SKIP_WARMUP" = true ]; then
    STEP_NUM=$((STEP_NUM - 1))
fi

echo "Step ${STEP_NUM}: Collecting benchmarks (${BENCH_COUNT} runs)"
cd "${SCRIPT_DIR}/../benchmarks"

# Disable automatic toolchain switching - use exact specified version
GOTOOLCHAIN=local "$GO_BIN" test -bench=. -benchmem -count=$BENCH_COUNT -benchtime=$BENCH_TIME ./core/ \
    | tee "$OUTPUT_FILE"

echo
echo -e "${GREEN}✓ Collection complete${NC}"
echo

cd "${SCRIPT_DIR}/.."

# Summary
echo "=== Collection Summary ==="
echo
echo "File:     $OUTPUT_FILE"
echo "Size:     $(du -h "$OUTPUT_FILE" | cut -f1)"
echo "Version:  $(GOTOOLCHAIN=local $GO_BIN version)"
echo
echo "To compare with another version:"
echo "  ./tools/compare-stable.sh <baseline-file> $OUTPUT_FILE"
