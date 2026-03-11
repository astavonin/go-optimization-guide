---
date:
  created: 2026-03-11
categories:
  - optimizations
  - performance tracking
---

# Go Performance Numbers You Can Actually Trace Back to Something

!!! warning
    All benchmarks on this site are synthetic. They measure isolated behavior of Go's runtime and standard library under controlled, artificial conditions. Your production workload has different allocation patterns, different concurrency, different GC pressure, and different hardware. A result that shows a 20% improvement in a tight benchmark loop may mean nothing in your service, or it may mean everything -- you won't know without profiling your own code. Use this data as a directional signal, not a deployment decision.

A performance guide without numbers is opinion dressed as engineering. I've been building [goperf.dev](https://goperf.dev) for a while now as a companion to my Go optimization work, and for most of that time the advice there was exactly that: well-reasoned opinion backed by understanding of the runtime internals, but not by reproducible measurements across Go versions. That changes now.

I've added actual benchmark data covering Go 1.24, 1.25, and 1.26 across three platforms: Linux amd64, Linux arm64, and macOS arm64. Seventy-six benchmarks spanning runtime internals, standard library, and networking. Every number on the site traces back to a specific EC2 instance type, a specific kernel version, a specific commit, and a documented collection process. The data lives on the [Go Version Performance Tracking](../../03-version-tracking/index.md) page.
<!-- more -->

## Why bother

The standard answer is "so you can compare Go versions." That's true but not the interesting part.

The more honest answer: without measurements, a performance guide can recommend things that are subtly wrong, or right for one version and wrong for the next. Go's runtime team ships meaningful changes between releases. GC behavior, Swiss maps landing in 1.24, scheduler changes. The qualitative advice ("prefer sync.Pool for allocations in hot paths") stays valid, but the *magnitude* shifts. You need numbers to catch when a recommendation stops paying off.

The second reason is discipline. Building a measurement pipeline forces you to understand what you're actually measuring, where the numbers come from, and where they don't apply. That understanding shapes what you write.

## What gets measured

Four benchmark packages, 76 benchmarks total.

`core` covers basic allocation patterns, maps, and GC pressure. `runtime` goes deeper into GC throughput and latency, Swiss map behavior, sync primitives, goroutine creation, and stack growth. `stdlib` covers JSON encoding and decoding, AES and SHA variants, RSA key generation, I/O, regexp, and binary encoding. `networking` covers TCP connection setup and throughput, TLS handshakes, session resumption, HTTP/2.

Benchmarks are organized as sub-benchmarks where size matters. `BenchmarkAESCTR` runs across 1KB, 64KB, and 1MB payloads. This matters: throughput characteristics change non-linearly with buffer size depending on cache behavior, and a single "average" number would hide that.

## How measurements are taken

Benchmarking on modern hardware is adversarial. The CPU changes clock frequency in response to load and temperature. The OS migrates threads between cores. The GC fires at inconvenient moments. Two identical runs of the same code on the same machine can produce results that differ by 5-30% without explicit controls in place.

The collection pipeline suppresses these sources of variance before any measurement starts. On Linux (EC2 `c6i.xlarge` for amd64, `c7g.xlarge` for arm64), the bootstrap script runs as root and applies the following:

The CPU governor is locked to `performance` mode, forcing all cores to their maximum P-state. Intel Turbo Boost is explicitly disabled -- it allows brief clock spikes above base frequency, but those spikes are unsustainable as heat accumulates, so a benchmark that catches a Turbo window looks faster than one that runs after thermal throttling kicks in. Deep C-states are disabled so the CPU stays fully awake between iterations instead of adding unpredictable wake-up latency. The page cache is dropped to ensure all runs start from the same memory state. Benchmarks are pinned to cores 2 and 3 via `taskset`, avoiding core 0 (which handles system interrupts) and preventing the OS scheduler from migrating threads mid-run.

A pre-run system check validates that the CPU governor is in `performance` mode, load average is below 2.0, CPU temperature is below 75°C, and available memory is above 1GB. If these checks fail, the run doesn't start.

Before the production run, a warmup pass runs 3 iterations at 1 second each to fill CPU caches. Those results are discarded entirely.

The production run: 20 iterations per benchmark, 3 seconds each. Twenty iterations is enough to compute meaningful mean, standard deviation, and coefficient of variation without pushing wall-clock time above 5-6 hours for the full suite across three Go versions. Three seconds per iteration lets even very fast benchmarks (sub-microsecond operations) accumulate millions of loop iterations for stable ns/op values.

All Go versions run sequentially on the same instance. Cross-version comparisons reflect only runtime differences, not hardware or environment differences.

### Variance and retries

After the initial run, the collector analyzes coefficient of variation (CV) for every benchmark. Anything above 15% CV gets flagged and re-run automatically with 30 iterations instead of 20. The pipeline retries up to twice. Benchmarks that remain above 15% CV after retries get recorded as unstable.

The final reliability classification uses the worst-case CV across all Go versions:

| Classification | CV Range |
|---|---|
| Reliable | < 5% |
| Noisy | 5-15% |
| Unstable | > 15% |

This is conservative by design. If a benchmark was stable in Go 1.24 but noisy in Go 1.26, it's classified as noisy overall. That tells you the benchmark *can* produce unstable results under the same infrastructure, even if it doesn't always.

### Where the platforms stand

| Platform | Total | Reliable | Noisy | Unstable |
|---|---|---|---|---|
| Linux amd64 | 76 | 59 (78%) | 10 (13%) | 7 (9%) |
| Linux arm64 | 76 | 72 (95%) | 2 (3%) | 2 (3%) |
| macOS arm64 | 76 | 73 (96%) | 2 (3%) | 1 (1%) |

The amd64 numbers are noisier. That's consistent with how Intel processors behave under benchmarking: deeper cache hierarchies, more speculative execution surface, more frequency scaling complexity even with the governor locked. `BenchmarkTCPConnect/Parallel` sits at 34.9% CV on amd64 and is basically useless for cross-version comparison. `BenchmarkRSAKeyGen/Bits4096` hits 33.7% -- RSA key generation involves randomness and variable-time algorithms, so this was expected. These benchmarks are still in the suite with their reliability labels visible rather than dropped. Dropping them would hide the signal that these workloads are inherently noisy in this environment.

macOS arm64 is collected locally on Apple Silicon hardware, not EC2, so the system-level controls don't apply. In practice, Apple Silicon's unified frequency domain tends to produce stable results anyway, which is what the numbers show.

## What the data doesn't tell you

This matters more than the methodology section, so read it before looking at any chart.

These are synthetic benchmarks. A benchmark that measures JSON decoding throughput runs a fixed payload in a tight loop with no other goroutines competing, no GC pressure from adjacent subsystems, no network jitter, no disk I/O. Your production service runs with 40 goroutines, a GC that fires mid-request, and a JSON payload that varies by two orders of magnitude depending on the query. The synthetic numbers tell you how Go's runtime and standard library behave in isolation.

A few specific gaps worth naming.

The amd64 data comes entirely from Intel Ice Lake (`c6i`). AMD Zen behaves differently, especially for crypto operations where AES-NI implementations differ and AVX-512 availability changes what the compiler generates. "Linux amd64" in this dataset means Intel Ice Lake specifically.

All instances are 4 vCPUs, benchmarks run at `GOMAXPROCS=4`. Production systems at 32 or 64 cores see qualitatively different GC pause distribution and scheduler contention. Nothing in this dataset speaks to that.

Networking benchmarks run over loopback. They measure Go's networking stack, not end-to-end network performance.

If a benchmark shows Go 1.26 is 15% faster at small allocations than 1.25 on Linux amd64, that's a real signal. If it shows 3%, that's within noise for most benchmarks. The reliability labels and CV values in the comparison tool are there to help you distinguish the two. Use them.

## What's next

The pipeline itself is open source, part of the [go-optimization-guide](https://github.com/astavonin/go-optimization-guide) repository. Everything needed to reproduce a run is in `perf-tracking/tools/`.

AMD Zen coverage is on the list. The `c7a.xlarge` (Zen 4) is a natural next addition. I want to quantify how much the microarchitecture differences actually affect the benchmark distributions before publishing those numbers, rather than leaving the explanation as an exercise for the reader.
