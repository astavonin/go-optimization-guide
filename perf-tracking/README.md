# Performance Tracking

System for tracking Go performance across releases.

## Structure

```
perf-tracking/
├── benchmarks/          # Benchmark test suites
│   └── core/            # Core benchmarks (allocation, GC, sync)
├── tools/               # Analysis and comparison tools
│   └── benchcompare/    # Result comparison tool
└── results/             # Benchmark results storage
    ├── raw/             # Raw JSON results by Go version (go1.24/, go1.25/, etc.)
    └── comparisons/     # Generated comparison reports
```

## Development Workflow

### Setting Up Go Versions

Install specific Go versions locally for benchmarking:

```bash
cd perf-tracking

# Install Go versions
./tools/setup-go-versions.sh install 1.24.0
./tools/setup-go-versions.sh install 1.25.0

# List installed versions
./tools/setup-go-versions.sh list

# Get path to specific version
./tools/setup-go-versions.sh path 1.24
```

Go versions are installed to `.go-versions/` (excluded from git).

### Linting

Before committing changes, run the linter to ensure code quality:

```bash
cd perf-tracking/benchmarks
./lint.sh
```

The linter runs:
- `go vet` - Basic Go static analysis
- `staticcheck` - Advanced static analysis
- `gopls check` - LSP diagnostics (includes additional analyzers)
- `gofmt` - Code formatting verification

### Running Benchmarks

Collect benchmark results with specific Go versions:

```bash
cd perf-tracking

# Collect results for Go 1.24
./tools/collect-local.sh 1.24

# Collect results for Go 1.25
./tools/collect-local.sh 1.25
```

Results are saved to `results/raw/goX.Y/` with metadata and benchmark data.

For manual testing:

```bash
cd perf-tracking/benchmarks

# Run all core benchmarks (uses system Go)
go test -bench=. -benchmem ./core/

# Run with specific version
$(./../tools/setup-go-versions.sh path 1.24) test -bench=. -benchmem ./core/
```

## Comparing Results

```bash
cd perf-tracking/tools/benchcompare
go build

./benchcompare \
    -baseline ../../results/raw/go1.24/results.json \
    -target ../../results/raw/go1.25/results.json \
    -output ../../results/comparisons/1.24-vs-1.25.md
```

## Result Formats

### Raw Results

File naming: `YYYY-MM-DD_HH-MM-SS_COMMIT.json`

```json
{
  "metadata": {
    "timestamp": "2024-01-01T10:00:00Z",
    "go_version": "go1.24",
    "go_version_full": "go version go1.24 linux/amd64",
    "commit_sha": "abc123",
    "runner": {
      "os": "linux",
      "arch": "amd64",
      "cores": 16
    }
  },
  "benchmarks": [...]
}
```

### Comparison Results

```json
{
  "baseline": {"go_version": "go1.24"},
  "target": {"go_version": "go1.25"},
  "results": [{
    "benchmark": "BenchmarkSmallAllocation",
    "delta_percent": -6.8,
    "direction": "faster"
  }]
}
```

## Benchmark Categories

- **memory** - Memory allocation patterns
- **gc** - Garbage collection behavior
- **stdlib** - Standard library performance
- **crypto** - Cryptographic operations
- **networking** - Network I/O and protocols
