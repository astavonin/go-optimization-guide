---
title: Go Version Performance Tracking
---

# Go Version Performance Tracking

!!! warning "Work in Progress"
    **This page and its benchmarks are currently under active development.**

    Recent quality improvements have been made to fix measurement accuracy issues (dead code elimination, interface boxing, throughput accounting). All benchmarks are being re-collected across Go versions to establish a new baseline.

    **Do not use this data for production decisions yet.** The interactive tool and results will be updated once the re-collection is complete.

Interactive tool to compare benchmark performance across Go releases.

## Interactive Comparison Tool

<iframe id="perf-iframe" src="interactive.html" width="100%" height="2000px" frameborder="0" style="border: 1px solid var(--md-default-fg-color--lighter); border-radius: 8px; margin: 20px 0;"></iframe>

<script>
// Sync theme with iframe
(function() {
    const iframe = document.getElementById('perf-iframe');
    if (!iframe) return;

    function updateIframeTheme() {
        const scheme = document.documentElement.getAttribute('data-md-color-scheme') || 'default';
        const currentSrc = iframe.src.split('?')[0];
        iframe.src = currentSrc + '?theme=' + scheme;
    }

    // Set initial theme
    updateIframeTheme();

    // Watch for theme changes
    const observer = new MutationObserver((mutations) => {
        mutations.forEach((mutation) => {
            if (mutation.attributeName === 'data-md-color-scheme') {
                // Send message to iframe instead of reloading
                iframe.contentWindow?.postMessage({ theme: document.documentElement.getAttribute('data-md-color-scheme') }, '*');
            }
        });
    });

    observer.observe(document.documentElement, {
        attributes: true,
        attributeFilter: ['data-md-color-scheme']
    });
})();
</script>

!!! tip "Full Screen Mode"
    [Open interactive tool in new window](interactive.html){ target="_blank" } for better visibility.

## Methodology

Benchmarks run with rigorous controls for consistency:

- **Iterations**: 20 runs per benchmark
- **Benchtime**: 3 seconds per iteration
- **Comparison**: Direct calculation of mean performance deltas between versions
- **System Validation**: CPU governor, load, and temperature checks before each run
- **Hardware**: Consistent CPU configuration across all runs

For CLI analysis with statistical significance testing, we use [`benchstat`](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat){ target="_blank" }.

Technical details: [perf-tracking documentation](https://github.com/astavonin/go-optimization-guide/tree/main/perf-tracking){ target="_blank" }

## Current Benchmark Suite

Core allocation patterns (5 benchmarks):

| Benchmark | Description | Category |
|-----------|-------------|----------|
| SmallAllocation | 64-byte allocation | Memory |
| LargeAllocation | 1MB allocation | Memory |
| MapAllocation | Map with 100 entries | Data Structures |
| SliceAppend | Slice growth (1000 appends) | Data Structures |
| GCPressure | Allocation under GC pressure | GC Behavior |

**Coming Soon**: Expanded coverage for stdlib, networking, and crypto (Phase 5).

## Key Findings

### Go 1.24 → 1.25
- **Overall**: Stable performance (no significant regressions)
- All benchmarks within statistical noise threshold

## About This Data

Results collected using:

- Go's standard `testing` package benchmarks
- Statistical validation with [`benchstat`](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat){ target="_blank" }
- Local runs on consistent hardware

All benchmark source code: [perf-tracking/benchmarks/](https://github.com/astavonin/go-optimization-guide/tree/main/perf-tracking/benchmarks){ target="_blank" }
