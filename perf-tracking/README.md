# Performance Tracking

Track Go performance across releases with statistical validation.

## Quick Start

```bash
cd perf-tracking

# 1. Install tools
./tools/install-tools.sh

# 2. Set up Go versions
./tools/setup-go-versions.sh install 1.24.0
./tools/setup-go-versions.sh install 1.25.0

# 3. Collect benchmarks
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

**`benchcompare`** - Export for web
```bash
cd tools/benchcompare && go build
./benchcompare --export-all \
  --results-dir ../../results/stable \
  --output-dir ../../../docs/03-version-tracking/data
```

## Structure

```
perf-tracking/
├── benchmarks/
│   ├── core/                # Benchmarks (5 total)
│   ├── go.mod               # Set to minimum Go version (1.23)
│   └── lint.sh              # Run linters
├── tools/
│   ├── benchcompare/        # Export tool
│   ├── setup-go-versions.sh # Go version management
│   ├── collect-stable.sh    # Benchmark collection
│   ├── system-check.sh      # System validation
│   └── install-tools.sh     # Install benchstat, staticcheck, gopls
└── results/stable/          # Results by version
    ├── go1.24/
    └── go1.25/
```

## Benchmarks

- **SmallAllocation** - 64-byte allocation
- **LargeAllocation** - 1MB allocation
- **MapAllocation** - Map with 100 entries
- **SliceAppend** - Slice growth (1000 appends)
- **GCPressure** - GC under allocation pressure

## Development

**Linting:**
```bash
cd benchmarks && ./lint.sh
```

**Manual testing:**
```bash
cd benchmarks
go test -bench=. -benchmem ./core/
```

## Web Visualization

Export data and serve locally:
```bash
# Export to JSON
./tools/benchcompare/benchcompare --export-all \
  --results-dir results/stable \
  --output-dir ../docs/03-version-tracking/data

# Serve docs
cd ../docs && mkdocs serve
# Open http://localhost:8000/03-version-tracking/interactive/
```

## Tips

**For stable results:**
- Close background apps
- Use `performance` CPU governor: `echo performance | sudo tee /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor`
- Check system load is low
- Avoid thermal throttling

**High variance (>10%)?**
- Increase iterations: `--count 30`
- Check CPU governor
- Verify system is idle
