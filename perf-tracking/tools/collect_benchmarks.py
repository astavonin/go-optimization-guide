#!/usr/bin/env python3
"""
Intelligent benchmark collection tool with variance checking and selective re-run.

This script performs benchmark collection with automatic variance validation and
can intelligently re-run only benchmarks that exceed variance thresholds.

Features:
- Variance threshold checking (CV = stddev/mean * 100%)
- Selective re-run of high-variance benchmarks only
- Sequential execution across multiple Go versions
- Progress tracking and detailed reporting
- Automatic result merging
"""

import argparse
import json
import os
import re
import subprocess
import sys
from dataclasses import dataclass
from datetime import datetime
from pathlib import Path
from typing import Dict, List, Optional, Tuple
import math
import statistics


# Variance thresholds (CV %)
VARIANCE_GOOD = 5.0
VARIANCE_ACCEPTABLE = 10.0
VARIANCE_WARNING = 15.0
VARIANCE_HIGH = 30.0


@dataclass
class BenchmarkResult:
    """Single benchmark iteration result."""
    name: str
    iterations: int
    ns_per_op: float
    bytes_per_op: Optional[int] = None
    allocs_per_op: Optional[int] = None


@dataclass
class BenchmarkStats:
    """Statistical analysis of benchmark runs."""
    name: str
    count: int
    mean: float
    stddev: float
    cv: float  # Coefficient of variation (%)
    min_val: float
    max_val: float

    @property
    def category(self) -> str:
        """Classify variance quality."""
        if self.cv < VARIANCE_GOOD:
            return "good"
        elif self.cv < VARIANCE_ACCEPTABLE:
            return "acceptable"
        elif self.cv < VARIANCE_WARNING:
            return "warning"
        elif self.cv < VARIANCE_HIGH:
            return "high"
        else:
            return "very_high"

    @property
    def passed(self) -> bool:
        """Check if variance is acceptable (< 15%)."""
        return self.cv < VARIANCE_WARNING


class BenchmarkParser:
    """Parse Go benchmark output and calculate statistics."""

    # Regex to match benchmark lines: BenchmarkName-N  iterations  ns/op  [bytes/op  allocs/op]
    BENCH_PATTERN = re.compile(
        r'^(Benchmark\S+)-\d+\s+(\d+)\s+(\d+(?:\.\d+)?)\s+ns/op'
        r'(?:\s+(\d+)\s+B/op)?(?:\s+(\d+)\s+allocs/op)?'
    )

    def parse_file(self, filepath: Path) -> List[BenchmarkResult]:
        """Parse benchmark results from output file."""
        results = []

        with open(filepath, 'r') as f:
            for line in f:
                match = self.BENCH_PATTERN.match(line)
                if match:
                    name, iterations, ns_op, bytes_op, allocs_op = match.groups()
                    results.append(BenchmarkResult(
                        name=name,
                        iterations=int(iterations),
                        ns_per_op=float(ns_op),
                        bytes_per_op=int(bytes_op) if bytes_op else None,
                        allocs_per_op=int(allocs_op) if allocs_op else None
                    ))

        return results

    def calculate_stats(self, results: List[BenchmarkResult]) -> Dict[str, BenchmarkStats]:
        """Calculate statistics for each unique benchmark."""
        # Group by benchmark name
        grouped: Dict[str, List[float]] = {}
        for result in results:
            if result.name not in grouped:
                grouped[result.name] = []
            grouped[result.name].append(result.ns_per_op)

        # Calculate statistics
        stats = {}
        for name, values in grouped.items():
            if len(values) < 2:
                continue  # Need at least 2 samples for variance

            mean = statistics.mean(values)
            stddev = statistics.stdev(values)
            cv = (stddev / mean * 100) if mean > 0 else 0

            stats[name] = BenchmarkStats(
                name=name,
                count=len(values),
                mean=mean,
                stddev=stddev,
                cv=cv,
                min_val=min(values),
                max_val=max(values)
            )

        return stats


class BenchmarkRunner:
    """Execute Go benchmarks with variance checking."""

    def __init__(self, script_dir: Path, verbose: bool = False):
        self.script_dir = script_dir
        self.benchmarks_dir = script_dir.parent / "benchmarks"
        self.results_dir = script_dir.parent / "results" / "stable"
        self.parser = BenchmarkParser()
        self.verbose = verbose

    def find_go_binary(self, version: str) -> Optional[Path]:
        """Find Go binary for specified version."""
        setup_script = self.script_dir / "setup-go-versions.sh"

        try:
            result = subprocess.run(
                [str(setup_script), "path", version],
                capture_output=True,
                text=True,
                check=False
            )

            if result.returncode == 0:
                go_bin = Path(result.stdout.strip())
                if go_bin.exists() and go_bin.is_file():
                    return go_bin
        except Exception as e:
            print(f"Error finding Go binary: {e}", file=sys.stderr)

        return None

    def run_system_check(self) -> bool:
        """Run system stability checks."""
        check_script = self.script_dir / "system-check.sh"

        print("Running system stability checks...")
        result = subprocess.run([str(check_script)], capture_output=True)

        if result.returncode == 0:
            print("✓ System checks passed")
            return True
        else:
            print("⚠ System checks have warnings")
            response = input("Continue anyway? (y/N): ")
            return response.lower() == 'y'

    def warmup(self, go_bin: Path, skip: bool = False) -> bool:
        """Run warmup iterations."""
        if skip:
            return True

        print("Running warmup (3 iterations)...")

        os.chdir(self.benchmarks_dir)

        cmd = [
            str(go_bin), "test",
            "-bench=.", "-benchmem",
            "-count=3", "-benchtime=1s",
            "-timeout=300s",
            "./runtime/", "./stdlib/", "./networking/"
        ]

        env = os.environ.copy()
        env["GOTOOLCHAIN"] = "local"

        result = subprocess.run(
            cmd,
            env=env,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL
        )

        if result.returncode == 0:
            print("✓ Warmup complete")
            return True
        else:
            print("⚠ Warmup had issues (continuing anyway)")
            return True

    def run_benchmarks(
        self,
        go_bin: Path,
        output_file: Path,
        count: int = 20,
        benchtime: str = "3s",
        benchmark_filter: Optional[str] = None
    ) -> bool:
        """Run benchmark suite."""
        os.chdir(self.benchmarks_dir)

        # Build benchmark command
        bench_arg = f"-bench={benchmark_filter}" if benchmark_filter else "-bench=."

        cmd = [
            str(go_bin), "test",
            bench_arg, "-benchmem",
            f"-count={count}",
            f"-benchtime={benchtime}",
            "-timeout=1800s",
            "./runtime/", "./stdlib/", "./networking/"
        ]

        env = os.environ.copy()
        env["GOTOOLCHAIN"] = "local"

        print(f"Running benchmarks ({count} iterations, {benchtime} each)...")
        if benchmark_filter:
            print(f"  Filter: {benchmark_filter}")

        # Run and capture output
        with open(output_file, 'w') as f:
            result = subprocess.run(
                cmd,
                env=env,
                stdout=subprocess.PIPE,
                stderr=subprocess.STDOUT,
                text=True
            )

            # Write to file and echo to console
            output = result.stdout
            f.write(output)

            if self.verbose:
                print(output)

        return result.returncode == 0

    def analyze_variance(
        self,
        output_file: Path,
        threshold: float = VARIANCE_WARNING
    ) -> Tuple[Dict[str, BenchmarkStats], List[str]]:
        """Analyze benchmark variance and identify failures."""
        results = self.parser.parse_file(output_file)
        stats = self.parser.calculate_stats(results)

        if not stats:
            print("⚠ Warning: No benchmark data found for variance analysis")
            return {}, []

        # Categorize results
        categories = {
            "good": [],
            "acceptable": [],
            "warning": [],
            "high": [],
            "very_high": []
        }

        for bench_stats in stats.values():
            categories[bench_stats.category].append(bench_stats)

        # Print summary
        print(f"\nVariance Analysis ({len(stats)} benchmarks):")
        print(f"  Good (CV < 5%):       {len(categories['good'])} benchmarks")
        print(f"  Acceptable (5-10%):   {len(categories['acceptable'])} benchmarks")
        print(f"  Warning (10-15%):     {len(categories['warning'])} benchmarks")

        high_count = len(categories['high'])
        very_high_count = len(categories['very_high'])

        if high_count > 0 or very_high_count > 0:
            print(f"  ⚠ High (15-30%):      {high_count} benchmarks")
            print(f"  ✗ Very High (> 30%):  {very_high_count} benchmarks")
        else:
            print(f"  High (15-30%):        {high_count} benchmarks")
            print(f"  Very High (> 30%):    {very_high_count} benchmarks")

        # List high-variance benchmarks
        failed = []
        high_variance = categories['high'] + categories['very_high']

        if high_variance:
            print(f"\nHigh-variance benchmarks (CV > {VARIANCE_WARNING}%):")
            for bench_stats in sorted(high_variance, key=lambda x: x.cv, reverse=True):
                severity = "unreliable" if bench_stats.cv > VARIANCE_HIGH else "high"
                print(f"  {bench_stats.name}: {bench_stats.cv:.1f}% CV ({severity})")

                if bench_stats.cv > threshold:
                    failed.append(bench_stats.name)

            print(f"\n⚠ Warning: {len(high_variance)} benchmark(s) have high variance")
            print("  Consider re-running with --count 30 for better stability")
        else:
            print(f"\n✓ All benchmarks have acceptable variance (<{VARIANCE_WARNING}%)")

        return stats, failed

    def merge_results(
        self,
        original_file: Path,
        rerun_file: Path,
        output_file: Path,
        rerun_benchmarks: List[str]
    ) -> None:
        """Merge re-run results with original, replacing high-variance benchmarks."""
        print(f"\nMerging results...")
        print(f"  Original: {original_file}")
        print(f"  Re-run:   {rerun_file}")
        print(f"  Output:   {output_file}")

        # Parse both files
        original_results = self.parser.parse_file(original_file)
        rerun_results = self.parser.parse_file(rerun_file)

        # Build set of re-run benchmark names
        rerun_set = set(rerun_benchmarks)

        # Filter: keep original results except for re-run benchmarks
        merged = [r for r in original_results if r.name not in rerun_set]
        merged.extend(rerun_results)

        # Write merged results
        with open(output_file, 'w') as f:
            # Copy header from original
            with open(original_file, 'r') as orig:
                for line in orig:
                    if line.startswith('goos:') or line.startswith('goarch:') or \
                       line.startswith('pkg:') or line.startswith('cpu:'):
                        f.write(line)
                    elif line.startswith('Benchmark'):
                        break

            # Write merged results
            for result in sorted(merged, key=lambda x: x.name):
                line = f"{result.name}-16\t{result.iterations}\t{result.ns_per_op:.2f} ns/op"
                if result.bytes_per_op is not None:
                    line += f"\t{result.bytes_per_op} B/op"
                if result.allocs_per_op is not None:
                    line += f"\t{result.allocs_per_op} allocs/op"
                f.write(line + "\n")

        print(f"✓ Merged {len(merged)} benchmark results")


def create_benchmark_filter(benchmark_names: List[str]) -> str:
    """Create Go benchmark filter regex from list of benchmark names."""
    # Extract base names (remove -N suffix if present)
    base_names = []
    for name in benchmark_names:
        # Remove CPU suffix like -16
        base = re.sub(r'-\d+$', '', name)
        base_names.append(base)

    # Create regex: ^(Name1|Name2|Name3)$
    return "^(" + "|".join(base_names) + ")$"


def main():
    parser = argparse.ArgumentParser(
        description="Intelligent Go benchmark collection with variance checking",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Collect benchmarks for Go 1.24 with variance checking
  %(prog)s 1.24

  # Collect with strict 10%% threshold
  %(prog)s 1.24 --variance-threshold 10

  # Re-run only failed benchmarks from previous run
  %(prog)s 1.24 --rerun-failed results/stable/go1.24/2025-01-25_10-30-00.txt

  # Collect multiple versions sequentially
  %(prog)s 1.23 1.24 1.25 --sequential
        """
    )

    parser.add_argument(
        "versions",
        nargs="+",
        help="Go version(s) to benchmark (e.g., 1.24, 1.25)"
    )

    parser.add_argument(
        "--count",
        type=int,
        default=20,
        help="Number of benchmark runs (default: 20)"
    )

    parser.add_argument(
        "--benchtime",
        default="3s",
        help="Benchmark time per iteration (default: 3s)"
    )

    parser.add_argument(
        "--variance-threshold",
        type=float,
        default=VARIANCE_WARNING,
        help=f"Maximum acceptable CV%% (default: {VARIANCE_WARNING})"
    )

    parser.add_argument(
        "--rerun-count",
        type=int,
        default=30,
        help="Iteration count for re-running failed benchmarks (default: 30)"
    )

    parser.add_argument(
        "--max-reruns",
        type=int,
        default=2,
        help="Maximum number of re-run attempts (default: 2)"
    )

    parser.add_argument(
        "--rerun-failed",
        type=Path,
        help="Re-run only high-variance benchmarks from previous result file"
    )

    parser.add_argument(
        "--skip-warmup",
        action="store_true",
        help="Skip warmup iterations"
    )

    parser.add_argument(
        "--skip-checks",
        action="store_true",
        help="Skip system stability checks"
    )

    parser.add_argument(
        "--sequential",
        action="store_true",
        help="Run versions sequentially (avoid system contention)"
    )

    parser.add_argument(
        "--verbose",
        action="store_true",
        help="Show detailed benchmark output"
    )

    args = parser.parse_args()

    # Setup paths
    script_dir = Path(__file__).parent.resolve()
    runner = BenchmarkRunner(script_dir, verbose=args.verbose)

    # Process each version
    for version in args.versions:
        print(f"\n{'='*60}")
        print(f"Go {version} Benchmark Collection")
        print('='*60)

        # Find Go binary
        go_bin = runner.find_go_binary(version)
        if not go_bin:
            print(f"✗ Error: Go {version} not found", file=sys.stderr)
            print(f"  Install with: ./tools/setup-go-versions.sh install <version>")
            continue

        print(f"Go binary: {go_bin}")

        # Create output directory
        output_dir = runner.results_dir / f"go{version}"
        output_dir.mkdir(parents=True, exist_ok=True)

        # Handle re-run mode
        if args.rerun_failed:
            print(f"\n=== Re-run Mode ===")
            print(f"Analyzing: {args.rerun_failed}")

            # Analyze original results
            stats, failed = runner.analyze_variance(args.rerun_failed, args.variance_threshold)

            if not failed:
                print("✓ No high-variance benchmarks to re-run")
                continue

            print(f"\nRe-running {len(failed)} high-variance benchmark(s)...")

            # Create benchmark filter
            bench_filter = create_benchmark_filter(failed)

            # Run re-collection
            timestamp = datetime.now().strftime("%Y-%m-%d_%H-%M-%S")
            rerun_file = output_dir / f"{timestamp}_rerun.txt"

            success = runner.run_benchmarks(
                go_bin,
                rerun_file,
                count=args.rerun_count,
                benchtime=args.benchtime,
                benchmark_filter=bench_filter
            )

            if not success:
                print("✗ Re-run failed", file=sys.stderr)
                continue

            # Analyze re-run results
            print("\nRe-run variance analysis:")
            rerun_stats, rerun_failed = runner.analyze_variance(rerun_file, args.variance_threshold)

            # Merge results
            merged_file = output_dir / f"{timestamp}_merged.txt"
            runner.merge_results(args.rerun_failed, rerun_file, merged_file, failed)

            print(f"\n✓ Re-run complete: {merged_file}")

        else:
            # Full collection mode
            # System checks
            if not args.skip_checks:
                if not runner.run_system_check():
                    print("Skipping due to system check failure")
                    continue

            # Warmup
            if not runner.warmup(go_bin, skip=args.skip_warmup):
                print("Warmup failed, continuing anyway...")

            # Run benchmarks
            timestamp = datetime.now().strftime("%Y-%m-%d_%H-%M-%S")
            output_file = output_dir / f"{timestamp}.txt"

            success = runner.run_benchmarks(
                go_bin,
                output_file,
                count=args.count,
                benchtime=args.benchtime
            )

            if not success:
                print("✗ Benchmark collection failed", file=sys.stderr)
                continue

            print(f"\n✓ Collection complete: {output_file}")

            # Analyze variance
            stats, failed = runner.analyze_variance(output_file, args.variance_threshold)

            # Auto-retry if failures and retries enabled
            retry_count = 0
            while failed and retry_count < args.max_reruns:
                retry_count += 1
                print(f"\n--- Retry {retry_count}/{args.max_reruns} ---")
                print(f"Re-running {len(failed)} high-variance benchmark(s) with {args.rerun_count} iterations...")

                bench_filter = create_benchmark_filter(failed)
                rerun_file = output_dir / f"{timestamp}_retry{retry_count}.txt"

                success = runner.run_benchmarks(
                    go_bin,
                    rerun_file,
                    count=args.rerun_count,
                    benchtime=args.benchtime,
                    benchmark_filter=bench_filter
                )

                if not success:
                    print("✗ Retry failed", file=sys.stderr)
                    break

                # Analyze retry
                print("\nRetry variance analysis:")
                retry_stats, retry_failed = runner.analyze_variance(rerun_file, args.variance_threshold)

                # Merge
                merged_file = output_dir / f"{timestamp}_merged_r{retry_count}.txt"
                runner.merge_results(output_file, rerun_file, merged_file, failed)

                # Update for next iteration
                output_file = merged_file
                failed = retry_failed

            if failed:
                print(f"\n⚠ Warning: {len(failed)} benchmark(s) still have high variance after {retry_count} retries")
                print("  Manual investigation recommended")
            else:
                print(f"\n✓ All benchmarks passed variance threshold (<{args.variance_threshold}%)")

    print(f"\n{'='*60}")
    print("Collection complete")
    print('='*60)


if __name__ == "__main__":
    main()
