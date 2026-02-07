# Performance Tracking

Track Go performance across releases with statistical variance validation and intelligent re-run capabilities.

**Coverage:** 76 benchmarks across runtime (20), stdlib (35), and networking (21)

## Quick Start

```bash
cd perf-tracking

# 1. Install tools
./tools/install-tools.sh

# 2. Set up Go versions
./tools/setup-go-versions.sh install 1.23.0
./tools/setup-go-versions.sh install 1.24.0
./tools/setup-go-versions.sh install 1.25.0

# 3. Collect benchmarks (runs versions sequentially by default)
./tools/collect_benchmarks.py 1.23 1.24 1.25 --progress

# 4. Export to JSON for web UI (use your platform directory)
cd tools/benchexport
go run . --export-all \
  --results-dir ../../results/stable/darwin-arm64 \
  --output-dir ../../../docs/03-version-tracking/data
```

## Tools

### Collection Tools

**`collect_benchmarks.py`** - Intelligent benchmark runner
```bash
# Basic collection with quality control
./tools/collect_benchmarks.py 1.24 --progress

# Multiple versions (runs sequentially by default)
./tools/collect_benchmarks.py 1.23 1.24 1.25 --progress

# Re-run ONLY failed benchmarks from a previous run
./tools/collect_benchmarks.py 1.24 \
  --rerun-failed results/stable/darwin-arm64/go1.24/2026-01-25_10-00-00_failed_benchmarks.txt \
  --rerun-count 50 \
  --max-reruns 3

# Production quality: strict threshold + auto-retry
./tools/collect_benchmarks.py 1.24 \
  --variance-threshold 10 \
  --max-reruns 5 \
  --rerun-count 30 \
  --progress
```

**Features:**
- Real-time streaming variance detection (CV calculation during run)
- Post-run variance analysis for quality assurance
- Automatic retry loop for high-variance benchmarks
- Creates `_failed_benchmarks.txt` for manual re-runs with different parameters
- Atomic retry result merging (successful retries update original file)
- Sequential multi-version collection (prevents system contention)
- Progress tracking with JSON status file

**`setup-go-versions.sh`** - Manage Go installations
```bash
./tools/setup-go-versions.sh install 1.24.0  # Install specific version
./tools/setup-go-versions.sh list            # List installed versions
./tools/setup-go-versions.sh path 1.24       # Get binary path
```

### Analysis Tools

**`benchexport`** - Export results to JSON for web UI
```bash
cd tools/benchexport
go run . --export-all \
  --results-dir ../../results/stable/darwin-arm64 \
  --output-dir ../../../docs/03-version-tracking/data
```

This automatically finds the latest main result file for each Go version (skips retry and failed_benchmarks files), detects the platform from benchmark metadata (`goos`/`goarch`), and exports to a platform-specific subdirectory:

```
data/
├── platforms.json              # Lists all available platforms
├── darwin-arm64/
│   ├── index.json              # Version index for this platform
│   ├── go1.23.json
│   ├── go1.24.json
│   └── go1.25.json
└── linux-amd64/
    ├── index.json
    └── ...
```

Running the tool multiple times for different platforms merges entries into `platforms.json`.

**`benchstat`** - Command-line comparison
```bash
benchstat baseline.txt new.txt                # Compare two files
benchstat -alpha 0.01 baseline.txt new.txt    # Stricter significance
```

**`system-check.sh`** - Pre-collection validation
```bash
./tools/system-check.sh  # Check CPU governor, load, temperature
```

## Structure

```
perf-tracking/
├── benchmarks/
│   ├── runtime/             # GC, sync, memory (20 benchmarks)
│   ├── stdlib/              # encoding, I/O, crypto, hash, text (35 benchmarks)
│   ├── networking/          # TCP, TLS, HTTP/2, gRPC, QUIC (21 benchmarks)
│   ├── go.mod.template      # Minimal template (go 1.23)
│   ├── go.mod.1.23.0        # Go 1.23 dependencies
│   ├── go.mod.1.24.0        # Go 1.24 dependencies
│   └── go.mod.1.25.0        # Go 1.25 dependencies
├── tools/
│   ├── collect_benchmarks.py      # Python benchmark runner
│   ├── test_collect_benchmarks.py # Unit tests
│   ├── system-check.sh            # System validation
│   ├── setup-go-versions.sh       # Go version management
│   ├── install-tools.sh           # Install benchstat, etc.
│   └── benchexport/               # JSON export tool
│       ├── export.go              # Main export logic
│       └── export_test.go         # 81 unit tests
├── .go-versions/
│   ├── go1.23.0/                  # Isolated Go 1.23.0 installation
│   ├── go1.24.0/                  # Isolated Go 1.24.0 installation
│   └── go1.25.0/                  # Isolated Go 1.25.0 installation
└── results/stable/                # Collected benchmark results
    ├── darwin-arm64/              # Platform: GOOS-GOARCH (auto-detected)
    │   ├── go1.23/
    │   │   ├── YYYY-MM-DD_HH-MM-SS.txt                  # Main result file (auto-updated with successful retries)
    │   │   ├── YYYY-MM-DD_HH-MM-SS_retry1.txt           # Retry attempt 1 results
    │   │   ├── YYYY-MM-DD_HH-MM-SS_retry2.txt           # Retry attempt 2 results
    │   │   └── YYYY-MM-DD_HH-MM-SS_failed_benchmarks.txt # List of benchmarks that still failed after retries
    │   ├── go1.24/
    │   └── go1.25/
    ├── linux-amd64/               # Another platform
    │   ├── go1.23/
    │   └── ...
    └── collection_progress.json               # Real-time progress tracking
```

## Benchmarks

**Total: 76 benchmarks** across three categories

**Runtime & Memory** (20 benchmarks in `runtime/`):
- GC: throughput, latency, small objects, mixed workload
- Maps: sync.Map, Swiss Tables, presizing, iteration, access patterns
- Goroutines: creation, stack growth, channel operations
- Concurrency: mutex contention, atomic operations
- Memory: small allocations, pooling, escape analysis

**Standard Library** (35 benchmarks in `stdlib/`):
- **Encoding:** JSON encode/decode, binary encoding, base64
- **I/O:** ReadAll, buffered I/O, WriteString
- **Crypto:** AES-CTR/GCM (1KB-1MB sizes), RSA keygen (2048/4096 bits)
- **Hashing:** SHA-1/256/512, SHA3-256, CRC32, FNV-1a, MD5
- **Text:** Regexp compile/match, string operations
- **Compression:** gzip, deflate

**Networking** (21 benchmarks in `networking/`):
- **TCP:** connect, keep-alive, throughput, parallel connections
- **TLS:** handshake (1.2/1.3/ECDHE), session resume, throughput
- **HTTP:** GET/POST requests, HTTP/2 (sequential/parallel/throughput)
- **Connection pooling:** cold/warm start, parallel access
- **Advanced:** gRPC unary/streaming, QUIC handshake/throughput

## Dependency Management

The collection tool automatically handles versioned go.mod templates:
- `go.mod.template` - Base template (go 1.23)
- `go.mod.1.23.0` - x/crypto v0.27.0 (Go 1.23 compatible)
- `go.mod.1.24.0` - x/crypto v0.47.0 (latest for Go 1.24+)
- `go.mod.1.25.0` - x/crypto v0.47.0 (latest for Go 1.25+)

Template is automatically selected based on Go version being tested.

## Common Workflows

### Production Benchmark Collection

For high-quality, reliable data:

```bash
# 1. System check
./tools/system-check.sh

# 2. Collect all versions with strict quality control
#    - Runs sequentially by default (prevents system contention)
#    - Progress tracking enabled
#    - Automatic retry up to 5 times for high-variance benchmarks
#    - Creates _failed_benchmarks.txt if issues persist after retries
./tools/collect_benchmarks.py 1.23 1.24 1.25 \
  --progress \
  --variance-threshold 10 \
  --max-reruns 5 \
  --count 25 \
  --rerun-count 30

# 3. If any benchmarks failed variance checks after retries:
#    Re-run with higher iteration count (platform dir is auto-detected)
./tools/collect_benchmarks.py 1.23 \
  --rerun-failed results/stable/darwin-arm64/go1.23/YYYY-MM-DD_failed_benchmarks.txt \
  --rerun-count 100 \
  --max-reruns 3

# 4. Export to JSON (specify the platform directory)
cd tools/benchexport
go run . --export-all \
  --results-dir ../../results/stable/darwin-arm64 \
  --output-dir ../../../docs/03-version-tracking/data
```

### Quick Development Iteration

For fast feedback during development:

```bash
# Quick collection with relaxed quality
./tools/collect_benchmarks.py 1.24 \
  --count 3 \
  --benchtime 1s \
  --skip-system-check \
  --variance-threshold 20
```

### Handling High Variance

If collection completes with high-variance benchmarks (creates `_failed_benchmarks.txt`):

```bash
# Re-run ONLY failed benchmarks with more iterations
# This is 10-100x faster than re-running entire collection
./tools/collect_benchmarks.py 1.24 \
  --rerun-failed results/stable/darwin-arm64/go1.24/2026-01-25_10-00-00_failed_benchmarks.txt \
  --rerun-count 100 \
  --max-reruns 5 \
  --variance-threshold 10

# Note: Successful results are automatically merged back into the original file
# No manual merging needed!
```

### CI/CD Integration

For continuous benchmarking in CI:

```bash
# Fast collection with relaxed quality requirements
./tools/collect_benchmarks.py 1.24 \
  --count 5 \
  --benchtime 1s \
  --variance-threshold 25 \
  --max-reruns 1 \
  --skip-system-check \
  --verbose
```

## Development

**Manual benchmark testing:**
```bash
cd benchmarks

# Copy appropriate go.mod for your Go version
cp go.mod.1.23.0 go.mod

# Run specific category
go test -bench=. -benchmem ./runtime/
go test -bench=. -benchmem ./stdlib/
go test -bench=. -benchmem ./networking/

# Run specific benchmark
go test -bench=BenchmarkGCThroughput ./runtime/
```

**Linting:**
```bash
cd benchmarks
staticcheck ./...
```

**Test Python tools:**
```bash
cd tools
python3 test_collect_benchmarks.py  # Unit tests
./collect_benchmarks.py --help       # Show help
```

## Web Visualization

Interactive UI for comparing Go versions with category filtering, charts, and variance analysis.

**Export data:**
```bash
cd tools/benchexport
go run . --export-all \
  --results-dir ../../results/stable/darwin-arm64 \
  --output-dir ../../../docs/03-version-tracking/data
```

**View locally:**
```bash
cd ../../docs/03-version-tracking
python3 -m http.server 8000
# Open http://localhost:8000/interactive.html
```

**Published:** https://goperf.dev/03-version-tracking/interactive.html

**Features:**
- **Platform selector:** Switch between platforms (e.g., macOS arm64, Linux amd64)
- **Category filtering:** Filter by Runtime (20), Stdlib (35), or Networking (21)
- Compare any two Go versions
- Interactive charts (execution time, memory allocations, performance delta)
- Variance indicators (Good/Acceptable/Warning/High)
- Mobile-responsive design
- XSS-protected data rendering
- System metadata display (CPU, OS, architecture)

## Variance & Data Quality

**Variance is measured using Coefficient of Variation (CV):**
```
CV = (stddev / mean) × 100%
```

**Classification:**

| Category | CV Range | Interpretation | Action |
|----------|----------|----------------|--------|
| Good | < 5% | Highly stable | ✓ Accept |
| Acceptable | 5-10% | Minor variance | ✓ Accept |
| Warning | 10-15% | Borderline | ⚠ Review |
| High | 15-30% | Problematic | ⚠ Re-run |
| Very High | > 30% | Unreliable | ✗ Must re-run |

**Typical CV by category:**
- Runtime/stdlib: Aim for < 5% (CPU-bound, very stable)
- Networking: Accept 10-15% (I/O-bound, inherently variable)

## Collection Workflow

**Automated retry and variance checking:**

1. **Initial collection**: Runs all 76 benchmarks with `--count` iterations
2. **Streaming variance detection**: Real-time CV calculation during benchmark execution
3. **Post-run variance analysis**: Comprehensive CV analysis of all results
4. **Automatic retry loop** (if `--max-reruns > 0`):
   - Re-runs ONLY high-variance benchmarks with `--rerun-count` iterations
   - Creates separate retry files: `_retry1.txt`, `_retry2.txt`, etc.
   - Repeats up to `--max-reruns` times
   - Stops early if variance improves
5. **Result handling**:
   - **Success case**: All benchmarks pass variance threshold → Original `.txt` file is final result
   - **Partial success**: Some benchmarks improved → Successful retries automatically merged into original `.txt` file
   - **Failure case**: Some benchmarks still fail after all retries → Creates `_failed_benchmarks.txt`

**File outputs:**
- `YYYY-MM-DD_HH-MM-SS.txt` - Main result file (updated with successful retries)
- `YYYY-MM-DD_HH-MM-SS_retry1.txt` - First retry attempt results
- `YYYY-MM-DD_HH-MM-SS_retry2.txt` - Second retry attempt results
- `YYYY-MM-DD_HH-MM-SS_failed_benchmarks.txt` - List of benchmarks that need manual attention (only created if failures persist)
- `collection_progress.json` - Real-time progress tracking with package statistics

**Using --rerun-failed:**

If a `_failed_benchmarks.txt` file is created, re-run with different parameters:

```bash
# Re-run with much higher iteration count
./tools/collect_benchmarks.py 1.24 \
  --rerun-failed results/stable/darwin-arm64/go1.24/YYYY-MM-DD_failed_benchmarks.txt \
  --rerun-count 100 \
  --max-reruns 3

# Success → Results merged back into original YYYY-MM-DD_HH-MM-SS.txt
# Backup created: YYYY-MM-DD_HH-MM-SS.txt.backup
```

## Tips for Stable Results

**System preparation:**
```bash
# 1. Check system conditions
./tools/system-check.sh

# 2. Set CPU governor to performance (requires sudo)
sudo cpupower frequency-set -g performance

# 3. Close background applications
# 4. Wait for CPU to cool down (< 65°C recommended)
```

**If high variance detected:**

**Option 1: Use automatic retry during collection** (recommended)
```bash
./tools/collect_benchmarks.py 1.24 \
  --variance-threshold 10 \
  --max-reruns 5 \
  --rerun-count 30 \
  --progress
```

**Option 2: Re-run failed benchmarks after collection** (fastest for persistent failures)
```bash
# If collection creates _failed_benchmarks.txt, re-run with higher iteration count
./tools/collect_benchmarks.py 1.24 \
  --rerun-failed results/stable/darwin-arm64/go1.24/2026-01-27_15-35-22_failed_benchmarks.txt \
  --rerun-count 100 \
  --max-reruns 3
```

**Common causes:**
- High CPU temperature (thermal throttling)
- High system load (competing processes)
- CPU frequency scaling enabled
- Swap usage (memory pressure)
- Parallel network operations (inherent variability)
