#!/usr/bin/env bash
# EC2 user-data script for automated benchmark collection.
# Runs as root. GO_VERSIONS is injected by run-benchmark.sh as the second
# line of the composed user-data payload:
#   export GO_VERSIONS="1.23.0 1.24.0 1.25.0"

REPO_DIR=/home/ec2-user/repo
LOG_FILE=/var/log/benchmark-bootstrap.log

# Pre-create log file as 644 so ec2-user can read it for SCP even when
# the process runs as root under a restrictive umask (e.g. 0077).
install -m 644 /dev/null "$LOG_FILE"

# Redirect all output to a dedicated log (readable via SSH for debugging)
exec > "$LOG_FILE" 2>&1

echo "=== Benchmark bootstrap started at $(date) ==="
echo "GO_VERSIONS=${GO_VERSIONS:-<not set>}"

# ---------------------------------------------------------------------------
# IMDSv2 self-termination helper
# ---------------------------------------------------------------------------
terminate_self() {
    local TOKEN INSTANCE_ID REGION
    TOKEN=$(curl -sf -X PUT "http://169.254.169.254/latest/api/token" \
        -H "X-aws-ec2-metadata-token-ttl-seconds: 21600" 2>/dev/null || true)
    INSTANCE_ID=$(curl -sf \
        -H "X-aws-ec2-metadata-token: ${TOKEN}" \
        "http://169.254.169.254/latest/meta-data/instance-id" 2>/dev/null || true)
    REGION=$(curl -sf \
        -H "X-aws-ec2-metadata-token: ${TOKEN}" \
        "http://169.254.169.254/latest/meta-data/placement/region" 2>/dev/null || echo "us-east-1")

    if [ -n "${INSTANCE_ID:-}" ]; then
        echo "→ Terminating instance $INSTANCE_ID in $REGION..."
        aws ec2 terminate-instances \
            --region "$REGION" \
            --instance-ids "$INSTANCE_ID" 2>/dev/null || true
    else
        echo "⚠ Could not determine instance ID, skipping termination"
    fi
}

# ---------------------------------------------------------------------------
# Cleanup trap — always runs on EXIT
# Writes the marker (success or failure reason), waits for SCP window,
# then terminates the instance.
# ---------------------------------------------------------------------------
BENCHMARK_RESULT="failed: unknown error"
WATCHDOG_PID=""

cleanup() {
    echo
    echo "=== Cleanup started at $(date) ==="
    echo "Result: $BENCHMARK_RESULT"
    echo "$BENCHMARK_RESULT" > /tmp/benchmark-done

    # Stop the watchdog — we will terminate ourselves
    [ -n "${WATCHDOG_PID}" ] && kill "$WATCHDOG_PID" 2>/dev/null || true

    # Hold the instance alive so the operator can SCP results
    echo "→ Waiting 300s for SCP download window..."
    sleep 300

    terminate_self
}

trap cleanup EXIT

# ---------------------------------------------------------------------------
# Server-side watchdog (7-hour hard limit)
# Started BEFORE set -euo pipefail so that a subshell failure does not
# kill the watchdog prematurely.
# ---------------------------------------------------------------------------
( sleep 25200 && echo "Watchdog: 7-hour limit reached, terminating" && terminate_self ) &
WATCHDOG_PID=$!

set -euo pipefail

# ---------------------------------------------------------------------------
# Step 2: Install system packages
# ---------------------------------------------------------------------------
echo
echo "=== Step 2: Installing packages ==="
BENCHMARK_RESULT="failed: package installation"

yum install -y git python3 jq util-linux kernel-tools
echo "✓ Packages installed"

# ---------------------------------------------------------------------------
# Step 3: System tuning for stable benchmark results
# ---------------------------------------------------------------------------
echo
echo "=== Step 3: System tuning ==="
BENCHMARK_RESULT="failed: system tuning"

# Set CPU frequency governor to performance (reduces clock variance)
if command -v cpupower &>/dev/null; then
    cpupower frequency-set -g performance 2>/dev/null || true
else
    for f in /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor; do
        echo performance > "$f" 2>/dev/null || true
    done
fi

# Disable Intel Turbo Boost to prevent thermal-driven frequency spikes
echo 1 > /sys/devices/system/cpu/intel_pstate/no_turbo 2>/dev/null || true

# Disable deep CPU idle states (reduces latency jitter)
if command -v cpupower &>/dev/null; then
    cpupower idle-set -D 0 2>/dev/null || true
fi

# Set Energy Performance Preference to performance (Intel only)
for f in /sys/devices/system/cpu/cpu*/cpufreq/energy_performance_preference; do
    echo performance > "$f" 2>/dev/null || true
done

# Increase open file descriptor limit for the current process tree
ulimit -n 65536

# Drop page cache to start benchmarks from a clean memory state
sync && echo 3 > /proc/sys/vm/drop_caches

echo "✓ System tuning complete"

# ---------------------------------------------------------------------------
# Step 4: Clone repository
# ---------------------------------------------------------------------------
echo
echo "=== Step 4: Cloning repository ==="
BENCHMARK_RESULT="failed: git clone"

git clone --depth=1 https://github.com/astavonin/go-optimization-guide.git "$REPO_DIR"
chown -R ec2-user:ec2-user "$REPO_DIR"
echo "✓ Repository cloned to $REPO_DIR"

# Record run metadata for reproducibility tracking
BENCHMARK_RESULT="failed: run metadata"
mkdir -p "$REPO_DIR/perf-tracking/results"
chown ec2-user:ec2-user "$REPO_DIR/perf-tracking/results"
{
    echo "run_date=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "go_versions=$GO_VERSIONS"
    echo "repo_commit=$(git -C $REPO_DIR rev-parse HEAD)"
    echo "kernel=$(uname -r)"
    echo "instance_type=$(curl -sf -H "X-aws-ec2-metadata-token: $(curl -sf -X PUT http://169.254.169.254/latest/api/token -H 'X-aws-ec2-metadata-token-ttl-seconds: 60')" http://169.254.169.254/latest/meta-data/instance-type 2>/dev/null || echo unknown)"
} > "$REPO_DIR/perf-tracking/results/run-metadata.txt"
echo "✓ Run metadata recorded"

# ---------------------------------------------------------------------------
# Step 5: Validate go.mod.template
# ---------------------------------------------------------------------------
echo
echo "=== Step 5: Validating go.mod.template ==="
BENCHMARK_RESULT="failed: missing go.mod.template"

if [ ! -f "$REPO_DIR/perf-tracking/benchmarks/go.mod.template" ]; then
    echo "✗ go.mod.template not found"
    exit 1
fi
echo "✓ go.mod.template present"

# ---------------------------------------------------------------------------
# Step 6: Install requested Go versions
# ---------------------------------------------------------------------------
echo
echo "=== Step 6: Installing Go versions: $GO_VERSIONS ==="
BENCHMARK_RESULT="failed: Go installation"

for v in $GO_VERSIONS; do
    echo "→ Installing Go $v..."
    runuser -l ec2-user -c "cd $REPO_DIR && perf-tracking/tools/setup-go-versions.sh install $v"
done
echo "✓ All Go versions installed"

# ---------------------------------------------------------------------------
# Step 7: Install Go performance tools
# install-tools.sh requires 'go' in PATH; use the first installed version.
# ---------------------------------------------------------------------------
echo
echo "=== Step 7: Installing Go tools ==="
BENCHMARK_RESULT="failed: Go tools installation"

FIRST_VERSION=$(echo "$GO_VERSIONS" | cut -d' ' -f1)
FIRST_GO=$(runuser -l ec2-user -c \
    "cd $REPO_DIR && perf-tracking/tools/setup-go-versions.sh path $FIRST_VERSION")
FIRST_GO_BIN=$(dirname "$FIRST_GO")
FIRST_GOPATH=$(runuser -l ec2-user -c "PATH=${FIRST_GO_BIN}:\$PATH go env GOPATH")

runuser -l ec2-user -c \
    "PATH=${FIRST_GO_BIN}:${FIRST_GOPATH}/bin:\$PATH $REPO_DIR/perf-tracking/tools/install-tools.sh"
echo "✓ Go tools installed"

# ---------------------------------------------------------------------------
# Step 8: Pre-benchmark system check
# ---------------------------------------------------------------------------
echo
echo "=== Step 8: System check ==="
BENCHMARK_RESULT="failed: system check"

# Wait for the 1-minute load average to settle after Go installation
echo "→ Waiting 90s for load average to settle after installations..."
sleep 90

runuser -l ec2-user -c "cd $REPO_DIR && perf-tracking/tools/system-check.sh"
echo "✓ System check passed"

# ---------------------------------------------------------------------------
# Step 9: Warmup pass (short runs to fill CPU caches / JIT warm-up paths)
# ---------------------------------------------------------------------------
echo
echo "=== Step 9: Warmup pass ==="
BENCHMARK_RESULT="failed: warmup run"

runuser -l ec2-user -c \
    "cd $REPO_DIR/perf-tracking && \
     taskset -c 2,3 python3 tools/collect_benchmarks.py \
         --count 3 --benchtime 1s --skip-system-check \
         $GO_VERSIONS"
echo "✓ Warmup complete"

# Clear warmup results so they do not contaminate the production data
rm -rf "$REPO_DIR/perf-tracking/results/stable/"

# ---------------------------------------------------------------------------
# Step 10: Full benchmark collection
# ---------------------------------------------------------------------------
echo
echo "=== Step 10: Benchmark collection ==="
BENCHMARK_RESULT="failed: benchmark collection"

runuser -l ec2-user -c \
    "cd $REPO_DIR/perf-tracking && \
     taskset -c 2,3 python3 tools/collect_benchmarks.py \
         --count 20 --benchtime 3s --skip-system-check --progress \
         --max-reruns 3 --rerun-count 40 \
         $GO_VERSIONS"

# ---------------------------------------------------------------------------
# Step 11: Mark success — cleanup trap fires on EXIT and writes the marker
# ---------------------------------------------------------------------------
BENCHMARK_RESULT="success"

# Copy run metadata into stable/ so it is retrieved with the benchmark results
cp "$REPO_DIR/perf-tracking/results/run-metadata.txt" \
   "$REPO_DIR/perf-tracking/results/stable/run-metadata.txt" 2>/dev/null || true

echo
echo "=== Benchmarks completed successfully at $(date) ==="
