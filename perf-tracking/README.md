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

# 3. Collect benchmarks
./tools/collect_benchmarks.py 1.23 1.24 1.25 --sequential

# 4. Export to JSON for web UI
cd tools/benchexport
go run . \
  --go1.23 ../../results/stable/go1.23/latest.txt \
  --go1.24 ../../results/stable/go1.24/latest.txt \
  --go1.25 ../../results/stable/go1.25/latest.txt \
  --output ../../../docs/03-version-tracking/data
```

## Tools

### Collection Tools

**`collect_benchmarks.py`** - Intelligent benchmark runner
```bash
# Basic collection with quality control
./tools/collect_benchmarks.py 1.24

# Multiple versions sequentially (prevents system contention)
./tools/collect_benchmarks.py 1.23 1.24 1.25 --sequential

# Re-run ONLY failed benchmarks (10-20x faster than full re-run)
./tools/collect_benchmarks.py 1.24 \
  --rerun-failed results/stable/go1.24/2026-01-25_10-00-00.txt \
  --rerun-count 50

# Production quality: strict threshold + auto-retry
./tools/collect_benchmarks.py 1.24 \
  --variance-threshold 12 \
  --max-reruns 2 \
  --rerun-count 30
```

**Features:**
- Automatic variance checking (CV calculation)
- Selective re-run of only high-variance benchmarks
- Automatic retry logic with configurable attempts
- Intelligent result merging (preserves good data)
- Sequential multi-version collection

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
go run . \
  --go1.23 ../../results/stable/go1.23/2026-01-25_merged.txt \
  --go1.24 ../../results/stable/go1.24/2026-01-25_merged.txt \
  --go1.25 ../../results/stable/go1.25/2026-01-25_merged.txt \
  --output ../../../docs/03-version-tracking/data
```

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
    ├── go1.23/
    │   ├── YYYY-MM-DD_HH-MM-SS.txt           # Initial collection
    │   ├── YYYY-MM-DD_HH-MM-SS_rerun.txt     # Re-run of failures
    │   └── YYYY-MM-DD_HH-MM-SS_merged.txt    # Final merged result
    ├── go1.24/
    └── go1.25/
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
./tools/collect_benchmarks.py 1.23 1.24 1.25 \
  --sequential \
  --variance-threshold 12 \
  --max-reruns 2 \
  --count 25 \
  --rerun-count 40

# 3. Export to JSON
cd tools/benchexport
go run . \
  --go1.23 ../../results/stable/go1.23/YYYY-MM-DD_merged.txt \
  --go1.24 ../../results/stable/go1.24/YYYY-MM-DD_merged.txt \
  --go1.25 ../../results/stable/go1.25/YYYY-MM-DD_merged.txt \
  --output ../../docs/03-version-tracking/data
```

### Quick Development Iteration

For fast feedback during development:

```bash
# Quick collection with relaxed quality
./tools/collect_benchmarks.py 1.24 \
  --count 10 \
  --skip-warmup \
  --skip-checks \
  --variance-threshold 20
```

### Handling High Variance

If initial collection shows high variance:

```bash
# Option 1: Re-run only failures (fastest)
./tools/collect_benchmarks.py 1.24 \
  --rerun-failed results/stable/go1.24/2026-01-25_10-00-00.txt \
  --rerun-count 60

# Option 2: Automatic retry with even more iterations
./tools/collect_benchmarks.py 1.24 \
  --rerun-failed results/stable/go1.24/2026-01-25_10-00-00.txt \
  --rerun-count 100 \
  --variance-threshold 10
```

### CI/CD Integration

For continuous benchmarking in CI:

```bash
# Fast collection, accept higher variance
./tools/collect_benchmarks.py 1.24 \
  --count 10 \
  --benchtime 1s \
  --skip-variance-check \
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
go run . \
  --go1.23 ../../results/stable/go1.23/2026-01-25_merged.txt \
  --go1.24 ../../results/stable/go1.24/2026-01-25_merged.txt \
  --go1.25 ../../results/stable/go1.25/2026-01-25_merged.txt \
  --output ../../docs/03-version-tracking/data
```

**View locally:**
```bash
cd ../../docs/03-version-tracking
python3 -m http.server 8000
# Open http://localhost:8000/interactive.html
```

**Published:** https://goperf.dev/03-version-tracking/interactive.html

**Features:**
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

**Option 1: Use Python tool's automatic retry** (easiest)
```bash
./tools/collect_benchmarks.py 1.24 \
  --variance-threshold 15 \
  --max-reruns 2 \
  --rerun-count 30
```

**Option 2: Selective re-run** (fastest for few failures)
```bash
# Re-run ONLY failed benchmarks with more iterations
./tools/collect_benchmarks.py 1.24 \
  --rerun-failed results/stable/go1.24/2026-01-25_10-00-00.txt \
  --rerun-count 50
```

**Common causes:**
- High CPU temperature (thermal throttling)
- High system load (competing processes)
- CPU frequency scaling enabled
- Swap usage (memory pressure)
- Parallel network operations (inherent variability)
