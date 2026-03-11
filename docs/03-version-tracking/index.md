---
title: Go Version Performance Tracking
---

# Go Version Performance Tracking

Benchmark performance across Go releases, collected on dedicated EC2 instances with controlled CPU configuration and automatic variance retry logic.

!!! warning
    All benchmarks are synthetic. Results reflect isolated runtime and library behavior under controlled conditions — not production application performance. Benchmarks classified as *noisy* or *unstable* should be treated as directional only.

## Methodology

Each benchmark run uses dedicated EC2 instances tuned for low variance:

- **Hardware**: `c6i.xlarge` (Intel Ice Lake) for amd64, `c7g.xlarge` (AWS Graviton3) for arm64
- **Iterations**: 20 runs × 3 seconds benchtime per benchmark
- **CPU controls**: governor locked to `performance`, Turbo Boost disabled, deep C-states disabled, benchmarks pinned to cores 2–3 via `taskset`
- **Variance retry**: benchmarks exceeding 15% CV are automatically re-run with 30 iterations (up to 3 retries)
- **Reliability classification**: each benchmark is labelled *reliable* (CV < 5%), *noisy* (5–15%), or *unstable* (> 15%) based on the worst CV observed across all versions

For detailed methodology, see [How We Measure](../blog/posts/goperf-measurements.md).

## Benchmark Suite

76 benchmarks across four packages:

| Package | Count | Focus |
|---------|------:|-------|
| `core` | 5 | Basic allocation patterns |
| `runtime` | ~20 | GC, Swiss maps, sync primitives, goroutines, stack growth |
| `stdlib` | ~25 | JSON, crypto (AES, SHA, RSA), I/O, regexp, binary encoding |
| `networking` | ~25 | TCP, TLS handshake/resume, HTTP/2, connection pools |

All benchmark source: [perf-tracking/benchmarks/](https://github.com/astavonin/go-optimization-guide/tree/main/perf-tracking/benchmarks){ target="_blank" }

## Platforms

| Platform | Instance | Go versions |
|----------|----------|-------------|
| Linux amd64 | `c6i.xlarge` | 1.24, 1.25, 1.26 |
| Linux arm64 | `c7g.xlarge` | 1.24, 1.25, 1.26 |
| macOS arm64 | Apple Silicon (local) | 1.24, 1.25, 1.26 |

## Key Findings

**Go 1.24**

- Swiss Tables hash map implementation: faster map insertions and lookups across all map sizes

**Go 1.25**

- TLS handshake throughput: cumulative ~58% improvement since Go 1.23 (TLS 1.3 fast path)

**Go 1.26**

- Small allocation specialization: measurable reduction in allocation latency for sub-32-byte objects
- `io.ReadAll`: ~2× throughput improvement on large reads
- RSA-4096 key generation: ~3× faster

## About This Data

- Source: Go's standard `testing` package with `b.Loop()` (Go 1.24+)
- Export: [`benchexport`](https://github.com/astavonin/go-optimization-guide/tree/main/perf-tracking/tools/benchexport){ target="_blank" } — computes per-benchmark mean, stddev, CV, and reliability classification
- Each result traces back to a specific EC2 instance type, kernel version, and repo commit

## Interactive Comparison Tool

!!! tip "Full Screen Mode"
    [Open interactive tool in new window](interactive.html){ target="_blank" } for better visibility.

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
