# Performance Tracking

Track Go performance across releases with statistical validation.

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
./tools/collect-stable.sh 1.23
./tools/collect-stable.sh 1.24
./tools/collect-stable.sh 1.25

# 4. Compare results
benchstat results/stable/go1.23/*.txt results/stable/go1.24/*.txt
```

## Tools

**`setup-go-versions.sh`** - Manage Go installations
```bash
./tools/setup-go-versions.sh install 1.24.0  # Install
./tools/setup-go-versions.sh list            # List installed
./tools/setup-go-versions.sh path 1.24       # Get path
```

**`collect-stable.sh`** - Collect benchmarks (20 iterations × 3s)
```bash
./tools/collect-stable.sh 1.24               # Collect with validation
./tools/collect-stable.sh 1.24 --skip-checks # Skip system checks
./tools/collect-stable.sh 1.24 --count 30    # More iterations
```

**`benchstat`** - Compare results
```bash
benchstat old.txt new.txt                    # Compare two files
benchstat -alpha 0.01 old.txt new.txt        # Stricter significance
```

**`benchexport`** - Export for web
```bash
cd tools/benchexport && go build
./benchexport --export-all \
  --results-dir ../../results/stable \
  --output-dir ../../../docs/03-version-tracking/data
```

## Structure

```
perf-tracking/
├── benchmarks/
│   ├── runtime/             # GC, sync, memory (12 benchmarks)
│   ├── stdlib/              # encoding, I/O, crypto, hash, text (12 benchmarks)
│   ├── networking/          # TCP, TLS, HTTP/2, pools (9 benchmarks)
│   ├── go.mod.template      # Minimal template (go 1.23)
│   ├── go.mod.1.23.0        # Go 1.23 dependencies
│   ├── go.mod.1.24.0        # Go 1.24 dependencies
│   └── go.mod.1.25.0        # Go 1.25 dependencies
├── tools/
│   ├── benchexport/         # Export tool
│   ├── setup-go-versions.sh # Go version management
│   ├── collect-stable.sh    # Benchmark collection (uses templates)
│   ├── system-check.sh      # System validation
│   └── install-tools.sh     # Install benchstat, staticcheck, gopls
├── .go-versions/
│   ├── go1.23.0/            # Isolated Go 1.23.0 installation
│   ├── go1.24.0/            # Isolated Go 1.24.0 installation
│   └── go1.25.0/            # Isolated Go 1.25.0 installation
└── results/stable/          # Results by version
    ├── go1.23/
    ├── go1.24/
    └── go1.25/
```

## Benchmarks

**Runtime & GC** (12 benchmarks in `runtime/`):
- GC throughput, latency, small objects, mixed workload
- sync.Map, Swiss Tables, map presizing, iteration
- Stack growth, small allocations, goroutine creation, mutex contention

**Standard Library** (12 benchmarks in `stdlib/`):
- JSON encode/decode, binary encoding
- I/O (ReadAll, buffered I/O)
- Crypto (AES-CTR, AES-GCM, SHA-1/256/512, SHA3-256, RSA keygen)
- Hash (CRC32, FNV-1a)
- Regexp (compile, match)

**Networking** (9 benchmarks in `networking/`):
- TCP: connect, keep-alive, throughput
- TLS: handshake (1.2/1.3/ECDHE), resume, throughput
- HTTP: request, HTTP/2 (sequential, parallel)
- Connection pooling (cold, warm, parallel)

## Dependency Management

`collect-stable.sh` automatically handles versioned go.mod templates:
- `go.mod.template` - Base template (go 1.23)
- `go.mod.1.23.0` - x/crypto v0.27.0 (Go 1.23 compatible)
- `go.mod.1.24.0` - x/crypto v0.47.0 (latest for Go 1.24+)
- `go.mod.1.25.0` - x/crypto v0.47.0 (latest for Go 1.25+)

Template is automatically selected based on Go version being tested.

## Development

**Manual testing:**
```bash
cd benchmarks

# Copy appropriate go.mod for your Go version
cp go.mod.1.23.0 go.mod

# Run specific category
go test -bench=. -benchmem ./runtime/
go test -bench=. -benchmem ./stdlib/
go test -bench=. -benchmem ./networking/
```

**Linting:**
```bash
cd benchmarks
staticcheck ./...
```

## Web Visualization

Interactive UI for comparing Go versions with charts and statistics.

**Export data:**
```bash
cd tools/benchexport && go build
./benchexport --export-all \
  --results-dir ../../results/stable \
  --output-dir ../../../docs/03-version-tracking/data
```

**View locally:**
```bash
cd docs/03-version-tracking
python3 -m http.server 8000
# Open http://localhost:8000/interactive.html
```

**Published:** https://goperf.dev/03-version-tracking/interactive.html

**Features:**
- Compare any two Go versions
- Interactive charts (time, allocations, performance delta)
- Statistical variance indicators
- System metadata display

## Tips

**For stable results:**
- Close background apps
- Use `performance` CPU governor: `echo performance | sudo tee /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor`
- Check system load is low
- Avoid thermal throttling
- Accept 10% CV for CPU-bound, 15% CV for networking benchmarks

**High variance (>10%)?**
- Increase iterations: `--count 30`
- Check CPU governor
- Verify system is idle
