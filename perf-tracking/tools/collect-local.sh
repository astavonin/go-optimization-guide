#!/bin/bash
# Collect benchmark results for a specific Go version

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

GO_VERSION=$1
if [ -z "$GO_VERSION" ]; then
    echo "Usage: $0 <go-version>"
    echo "Example: $0 1.24"
    exit 1
fi

# Find Go binary for the specified version
GO_BIN=$("${SCRIPT_DIR}/setup-go-versions.sh" path "$GO_VERSION" 2>/dev/null)
if [ -z "$GO_BIN" ] || [ ! -x "$GO_BIN" ]; then
    echo "Error: Go version ${GO_VERSION} not found"
    echo "Install it first with: ./tools/setup-go-versions.sh install <full-version>"
    echo "Example: ./tools/setup-go-versions.sh install 1.24.0"
    exit 1
fi

echo "Using Go binary: ${GO_BIN}"

TIMESTAMP=$(date +"%Y-%m-%d_%H-%M-%S")
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "local")
OUTPUT_DIR="results/raw/go${GO_VERSION}"
OUTPUT_FILE="${OUTPUT_DIR}/${TIMESTAMP}_${COMMIT}.json"

mkdir -p "$OUTPUT_DIR"

echo "=== Collecting results for Go ${GO_VERSION} ==="
echo "Output: ${OUTPUT_FILE}"

# Run benchmarks with JSON output (newline-delimited JSON events)
cd benchmarks
"$GO_BIN" test -bench=. -benchmem -count=10 -benchtime=3s -json ./core/ > "../${OUTPUT_FILE}.jsonl"

# Extract benchmark output lines and add metadata
# Use jq --arg to safely pass variables with special characters
GO_VERSION_FULL=$("$GO_BIN" version)
TIMESTAMP_ISO=$(date -Iseconds)
OS_NAME=$(uname -s)
ARCH_NAME=$(uname -m)
CORES=$(nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 1)

cat "../${OUTPUT_FILE}.jsonl" | jq -s \
  --arg timestamp "$TIMESTAMP_ISO" \
  --arg go_ver "go${GO_VERSION}" \
  --arg go_ver_full "$GO_VERSION_FULL" \
  --arg commit "$COMMIT" \
  --arg os "$OS_NAME" \
  --arg arch "$ARCH_NAME" \
  --argjson cores "$CORES" \
'{
  metadata: {
    timestamp: $timestamp,
    go_version: $go_ver,
    go_version_full: $go_ver_full,
    commit_sha: $commit,
    runner: {
      os: $os,
      arch: $arch,
      cores: $cores
    }
  },
  benchmarks: [
    .[] |
    select(.Action == "output" and (.Output | test("Benchmark"))) |
    .Output
  ]
}' > "../${OUTPUT_FILE}"

echo "Results saved to: ${OUTPUT_FILE}"
cd ..
