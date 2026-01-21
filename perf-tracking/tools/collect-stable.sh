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

# Normalize version: if only major.minor (e.g., 1.23), append .0 for dependency files
GO_VERSION_FULL="$GO_VERSION"
if [[ "$GO_VERSION_FULL" =~ ^[0-9]+\.[0-9]+$ ]]; then
    GO_VERSION_FULL="${GO_VERSION_FULL}.0"
fi

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
    GOTOOLCHAIN=local "$GO_BIN" test -bench=. -benchmem -count=$WARMUP_COUNT -benchtime=1s ./runtime/ ./stdlib/ > /dev/null 2>&1

    echo -e "${GREEN}✓ System warmed up${NC}"
    echo

    cd "${SCRIPT_DIR}/.."
fi

# Function to prepare version-specific dependencies
prepare_dependencies() {
    local GO_VERSION=$1
    local GO_BIN=$2
    local CACHED_GOMOD="go.mod.${GO_VERSION}"
    local CACHED_GOSUM="go.sum.${GO_VERSION}"

    if [ -f "$CACHED_GOMOD" ] && [ -f "$CACHED_GOSUM" ]; then
        echo "  → Using cached dependencies for Go ${GO_VERSION}"
        cp "$CACHED_GOMOD" go.mod
        cp "$CACHED_GOSUM" go.sum
    else
        echo "  → Resolving latest dependencies for Go ${GO_VERSION}..."
        if [ ! -f "go.mod.template" ]; then
            echo "  ✗ go.mod.template not found"
            return 1
        fi

        cp go.mod.template go.mod

        # Resolve latest compatible dependencies
        echo "  → Running 'go get -u ./...'"
        if ! GOTOOLCHAIN=local "$GO_BIN" get -u ./...; then
            echo "  ✗ Failed to resolve dependencies"
            return 1
        fi

        echo "  → Running 'go mod tidy'"
        if ! GOTOOLCHAIN=local "$GO_BIN" mod tidy; then
            echo "  ✗ Failed to tidy dependencies"
            return 1
        fi

        # Cache for future runs
        cp go.mod "$CACHED_GOMOD"
        cp go.sum "$CACHED_GOSUM"
        echo -e "  ${GREEN}✓ Cached dependencies to $CACHED_GOMOD and $CACHED_GOSUM${NC}"
    fi
}

# Step 3: Prepare go.mod for this Go version
STEP_NUM=3
if [ "$SKIP_CHECKS" = true ]; then
    STEP_NUM=$((STEP_NUM - 1))
fi
if [ "$SKIP_WARMUP" = true ]; then
    STEP_NUM=$((STEP_NUM - 1))
fi

echo "Step ${STEP_NUM}: Preparing dependencies for Go ${GO_VERSION}"
cd "${SCRIPT_DIR}/../benchmarks"

# Backup current go.mod
if [ -f "go.mod" ]; then
    cp go.mod go.mod.backup
fi

# Prepare version-specific dependencies
if ! prepare_dependencies "$GO_VERSION_FULL" "$GO_BIN"; then
    echo -e "${YELLOW}⚠ Failed to prepare dependencies, using existing go.mod${NC}"
    if [ -f "go.mod.backup" ]; then
        mv go.mod.backup go.mod
    fi
fi
echo

# Step 4: Collect benchmarks
STEP_NUM=$((STEP_NUM + 1))

echo "Step ${STEP_NUM}: Collecting benchmarks (${BENCH_COUNT} runs)"

# Disable automatic toolchain switching - use exact specified version
GOTOOLCHAIN=local "$GO_BIN" test -bench=. -benchmem -count=$BENCH_COUNT -benchtime=$BENCH_TIME \
    ./runtime/ ./stdlib/ 2>&1 | tee "$OUTPUT_FILE"

# Restore original go.mod
if [ -f "go.mod.backup" ]; then
    mv go.mod.backup go.mod
fi

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
