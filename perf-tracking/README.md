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

## Running Benchmarks

```bash
cd perf-tracking/benchmarks

# Run all core benchmarks
go test -bench=. -benchmem ./core/

# Multiple iterations for statistical significance
go test -bench=. -benchmem -count=10 -benchtime=3s ./core/

# Specific benchmark
go test -bench=BenchmarkSmallAllocation -benchmem ./core/

# JSON output for processing
go test -bench=. -benchmem -json ./core/ > ../results/raw/go1.24/results.json
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
