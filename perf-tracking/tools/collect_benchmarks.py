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
import time
import shutil


@dataclass
class PackageSection:
    """Represents a package section in benchmark output."""
    header_lines: List[str]      # goos, goarch, pkg, cpu
    benchmark_lines: List[Tuple[str, List[str]]]  # [(name, result lines), ...]
    footer_lines: List[str]      # PASS, ok lines


@dataclass
class BenchmarkFile:
    """Parsed benchmark output file."""
    sections: List[PackageSection]


# Variance thresholds (CV %)
VARIANCE_GOOD = 5.0
VARIANCE_ACCEPTABLE = 10.0
VARIANCE_WARNING = 15.0
VARIANCE_HIGH = 30.0


@dataclass
class SubprocessResult:
    """Wrapper for subprocess execution results."""
    returncode: int
    stdout: str = ""


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


class ProgressTracker:
    """Track and report collection progress."""

    def __init__(self, progress_file: Path, versions: List[str]):
        self.progress_file = progress_file
        self.versions = versions
        self.state = {
            'started': datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
            'versions_total': len(versions),
            'versions': {v: {'status': 'pending', 'phase': None, 'benchmarks': 0} for v in versions},
            'current_version': None,
            'current_phase': None,
            'updated': None
        }
        self._save()

    def _timestamp(self) -> str:
        """Get current timestamp string."""
        return datetime.now().strftime("%H:%M:%S")

    def _save(self):
        """Save progress to file."""
        self.state['updated'] = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
        with open(self.progress_file, 'w') as f:
            json.dump(self.state, f, indent=2)

    def log(self, message: str):
        """Print message with timestamp."""
        print(f"[{self._timestamp()}] {message}")

    def start_version(self, version: str):
        """Mark version as started."""
        self.state['current_version'] = version
        self.state['versions'][version]['status'] = 'running'
        self.state['versions'][version]['started'] = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
        self._save()
        self.log(f"Go {version} - Starting collection")

    def set_phase(self, version: str, phase: str):
        """Update current phase."""
        self.state['current_phase'] = phase
        self.state['versions'][version]['phase'] = phase
        self._save()
        self.log(f"Go {version} - Phase: {phase}")

    def update_package(self, version: str, package: str, status: str, benchmarks: int = 0,
                       passed: int = 0, needed_retry: int = 0):
        """Update package status with detailed metrics.

        Args:
            needed_retry: Count of benchmarks that initially failed variance checks and needed
                         retry attempts. This is a historical count from initial run and does
                         NOT reflect final status after retries complete.
        """
        if 'packages' not in self.state['versions'][version]:
            self.state['versions'][version]['packages'] = {}

        # Only include metrics when package completes (done/failed)
        # For 'running' status, just track status without misleading zero counts
        package_info = {'status': status}
        if status in ('done', 'failed'):
            package_info['benchmarks'] = benchmarks
            package_info['passed'] = passed
            package_info['needed_retry'] = needed_retry

        self.state['versions'][version]['packages'][package] = package_info
        self._save()

    def complete_version(self, version: str, benchmarks: int, success: bool = True):
        """Mark version as complete."""
        self.state['versions'][version]['status'] = 'complete' if success else 'failed'
        self.state['versions'][version]['benchmarks'] = benchmarks
        self.state['versions'][version]['completed'] = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
        # Clear transient fields
        self.state['current_version'] = None
        self.state['current_phase'] = None
        self._save()

        status = "✓ Complete" if success else "✗ Failed"
        self.log(f"Go {version} - {status} ({benchmarks} benchmarks)")

    def get_summary(self) -> str:
        """Get progress summary."""
        completed = sum(1 for v in self.state['versions'].values() if v['status'] == 'complete')
        return f"{completed}/{self.state['versions_total']} versions completed"


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


class StreamingBenchmarkRunner:
    """Execute benchmarks with real-time progress reporting."""

    # Regex to detect benchmark start lines
    BENCHMARK_PATTERN = re.compile(r'^(Benchmark\w+(?:/\w+)*)-\d+')
    # Regex to detect benchmark failures (can be at start or after tab)
    FAIL_PATTERN = re.compile(r'--- FAIL: (Benchmark\w+(?:/\w+)*)')

    def __init__(self, progress: Optional[ProgressTracker] = None, verbose: bool = False,
                 variance_threshold: float = 15.0):
        self.progress = progress
        self.verbose = verbose
        self.variance_threshold = variance_threshold
        self.current_benchmark = None
        self.benchmark_count = 0
        self.current_benchmark_status = None
        self.failed_benchmarks = []  # Test crashes
        self.high_variance_benchmarks = []  # Early variance detection

        # Track running statistics for current benchmark
        self.current_benchmark_runs = []
        self.current_run_count = 0

    def reset_for_package(self):
        """Reset benchmark tracking state for a new package."""
        self.benchmark_count = 0
        self.current_benchmark = None
        self.current_benchmark_status = None
        self.failed_benchmarks = []
        self.high_variance_benchmarks = []
        self.current_benchmark_runs = []
        self.current_run_count = 0

    def run_with_streaming(
        self,
        cmd: List[str],
        env: dict,
        package_name: str
    ) -> Tuple[int, str, List[str]]:
        """Run command with real-time output streaming.

        Returns:
            (returncode, full_output_string, failed_benchmarks_list)
        """
        if self.verbose:
            print(f"    Executing: {' '.join(cmd)}")

        output_lines = []

        # Use line-buffered Popen for real-time streaming
        proc = subprocess.Popen(
            cmd,
            env=env,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            bufsize=1  # Line buffered
        )

        try:
            # Read output line by line
            for line in proc.stdout:
                output_lines.append(line)

                # Check if this is a benchmark start line
                match = self.BENCHMARK_PATTERN.match(line)
                if match:
                    benchmark_name = match.group(1)

                    # Check if this is a NEW benchmark (different from current)
                    if benchmark_name != self.current_benchmark:
                        # Report completion of previous benchmark
                        if self.current_benchmark:
                            self._report_benchmark_complete()

                        # Start tracking new benchmark
                        self.current_benchmark = benchmark_name
                        self.current_benchmark_status = 'running'
                        self.benchmark_count += 1
                        self.current_benchmark_runs = []
                        self.current_run_count = 0

                    # Track this run of the current benchmark
                    self.current_run_count += 1

                    # Check if this line has a FAIL marker (test crash)
                    if '--- FAIL:' in line:
                        self.current_benchmark_status = 'crashed'
                        if benchmark_name not in self.failed_benchmarks:
                            self.failed_benchmarks.append(benchmark_name)
                    # Extract timing value for variance tracking
                    elif 'ns/op' in line:
                        ns_per_op = self._extract_timing(line)
                        if ns_per_op:
                            self.current_benchmark_runs.append(ns_per_op)
                            # Check variance after we have enough samples
                            if len(self.current_benchmark_runs) >= 5:
                                cv = self._calculate_cv(self.current_benchmark_runs)
                                if cv > self.variance_threshold:
                                    self.current_benchmark_status = 'high_variance'

                    # Show progress update
                    self._report_benchmark_progress()

            # Report final benchmark completion
            if self.current_benchmark:
                self._report_benchmark_complete()

            proc.wait()
            full_output = ''.join(output_lines)

            # Combine all failures
            all_failed = list(set(self.failed_benchmarks + self.high_variance_benchmarks))
            return proc.returncode, full_output, all_failed
        finally:
            # Ensure process cleanup even on exception
            if proc.poll() is None:
                proc.terminate()
                try:
                    proc.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    proc.kill()
                    proc.wait()
            if proc.stdout:
                proc.stdout.close()

    def _extract_timing(self, line: str) -> Optional[float]:
        """Extract ns/op timing value from benchmark line."""
        try:
            # Format: "BenchmarkName-16    	  12345	  123.45 ns/op"
            parts = line.split()
            for i, part in enumerate(parts):
                if part == 'ns/op' and i > 0:
                    return float(parts[i-1])
        except (ValueError, IndexError):
            pass
        return None

    def _calculate_cv(self, values: List[float]) -> float:
        """Calculate coefficient of variation (CV%)."""
        if len(values) < 2:
            return 0.0
        mean = statistics.mean(values)
        if mean == 0:
            return 0.0
        stddev = statistics.stdev(values)
        return (stddev / mean) * 100

    def _report_benchmark_progress(self):
        """Report progress during benchmark runs (compact updates)."""
        if not self.current_benchmark:
            return

        # Determine status indicator
        if self.current_benchmark_status == 'crashed':
            status = '✗ CRASH'
        else:
            cv = self._calculate_cv(self.current_benchmark_runs)
            cv_str = f' CV={cv:.1f}%' if cv > 0 else ''
            status = f'[{self.current_run_count}]{cv_str}'

        if self.progress:
            # With progress tracker, only update on first run
            if self.current_run_count == 1:
                self.progress.log(f"    → {self.current_benchmark}")
        elif self.verbose:
            # Verbose: show each iteration
            print(f"\r    {self.current_benchmark} {status}", end="", flush=True)
        else:
            # Compact: update same line
            print(f"\r    {self.current_benchmark[:50]:<50} {status}", end="", flush=True)

    def _report_benchmark_complete(self):
        """Report completion of benchmark (all runs done)."""
        if not self.current_benchmark:
            return

        # Calculate final statistics
        cv = self._calculate_cv(self.current_benchmark_runs) if self.current_benchmark_runs else 0

        # Determine final status
        if self.current_benchmark_status == 'crashed':
            status_str = f'✗ CRASHED ({self.current_run_count} runs)'
        elif cv > self.variance_threshold:
            status_str = f'⚠ HIGH VARIANCE ({self.current_run_count} runs, CV={cv:.1f}%)'
            if self.current_benchmark not in self.high_variance_benchmarks:
                self.high_variance_benchmarks.append(self.current_benchmark)
        else:
            status_str = f'✓ ({self.current_run_count} runs, CV={cv:.1f}%)'

        if self.progress:
            self.progress.log(f"    [{self.benchmark_count}] {self.current_benchmark} {status_str}")
        elif self.verbose:
            print(f"\r    [{self.benchmark_count}] {self.current_benchmark} {status_str}")
        else:
            # Compact: clear line and show final status
            print(f"\r{' ' * 80}\r    [{self.benchmark_count}] {self.current_benchmark[:40]:<40} {status_str}")


class BenchmarkRunner:
    """Execute Go benchmarks with variance checking."""

    def __init__(self, script_dir: Path, verbose: bool = False, progress: Optional[ProgressTracker] = None,
                 variance_threshold: float = 15.0):
        self.script_dir = script_dir
        self.benchmarks_dir = script_dir.parent / "benchmarks"
        self.results_base_dir = script_dir.parent / "results" / "stable"
        self.results_dir = None  # Set by detect_platform()
        self.parser = BenchmarkParser()
        self.verbose = verbose
        self.progress = progress
        self.variance_threshold = variance_threshold
        self.streaming_runner = StreamingBenchmarkRunner(progress, verbose, variance_threshold)

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

    def detect_platform(self, go_bin: Path) -> str:
        """Detect GOOS-GOARCH from Go binary and set results_dir accordingly.

        Returns the platform string (e.g. 'darwin-arm64').
        """
        env = os.environ.copy()
        env["GOTOOLCHAIN"] = "local"

        result = subprocess.run(
            [str(go_bin), "env", "GOOS", "GOARCH"],
            capture_output=True, text=True, check=False, env=env
        )
        if result.returncode != 0:
            raise RuntimeError(f"Failed to detect platform: {result.stderr.strip()}")

        lines = result.stdout.strip().splitlines()
        if len(lines) != 2:
            raise RuntimeError(f"Unexpected go env output: {result.stdout.strip()}")

        platform = f"{lines[0]}-{lines[1]}"
        self.results_dir = self.results_base_dir / platform
        return platform

    def prepare_dependencies(self, version: str) -> bool:
        """Prepare version-specific go.mod dependencies."""
        # Normalize version (e.g., "1.23" -> "1.23.0")
        version_full = version
        if version.count('.') == 1:
            version_full = f"{version}.0"

        cached_gomod = self.benchmarks_dir / f"go.mod.{version_full}"
        cached_gosum = self.benchmarks_dir / f"go.sum.{version_full}"

        target_gomod = self.benchmarks_dir / "go.mod"
        target_gosum = self.benchmarks_dir / "go.sum"

        # Backup current go.mod
        backup_gomod = self.benchmarks_dir / "go.mod.backup"
        if target_gomod.exists():
            shutil.copy(target_gomod, backup_gomod)

        # Use cached version-specific files if available
        if cached_gomod.exists() and cached_gosum.exists():
            print(f"  → Using cached dependencies for Go {version_full}")
            shutil.copy(cached_gomod, target_gomod)
            shutil.copy(cached_gosum, target_gosum)
            return True
        else:
            print(f"  ✗ Cached go.mod.{version_full} not found")
            # Restore backup if copy failed
            if backup_gomod.exists():
                shutil.copy(backup_gomod, target_gomod)
            return False

    def run_system_check(self, skip: bool = False) -> bool:
        """Run system stability checks.

        Args:
            skip: If True, skip checks entirely (for CI/CD)
        """
        if skip:
            print("Skipping system stability checks (--skip-system-check)")
            return True

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

    def warmup(self, go_bin: Path, skip: bool = False, version: str = None) -> bool:
        """Run warmup iterations."""
        if skip:
            return True

        if self.progress and version:
            self.progress.set_phase(version, "Warmup")
        else:
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

        if self.progress and version:
            # Use streaming runner for warmup progress
            self.progress.log("Running warmup benchmarks...")
            returncode, _, _ = self.streaming_runner.run_with_streaming(cmd, env, "warmup")
            result = SubprocessResult(returncode=returncode)
        else:
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
        benchmark_filter: Optional[str] = None,
        benchmark_filters: Optional[List[str]] = None,
        test_packages: Optional[List[str]] = None,
        version: str = None,
        is_retry: bool = False
    ) -> Dict[str, any]:
        """Run benchmark suite.

        Args:
            benchmark_filter: Single filter string (legacy, converted to list internally)
            benchmark_filters: List of filter strings. When multiple filters are
                provided, each is run as a separate go test invocation per package
                to avoid Go's -bench regex limitations with mixed subtest patterns.
            is_retry: If True, skip updating package-level progress (retries don't change totals)

        Returns dict with:
        - success: bool
        - results: dict of {package: {'success': bool, 'error': str, 'benchmarks': int}}
        - output_file: Path
        """
        os.chdir(self.benchmarks_dir)

        if test_packages is None:
            test_packages = ["runtime", "stdlib", "networking"]

        # Normalize to list of filters
        if benchmark_filters is None:
            benchmark_filters = [benchmark_filter] if benchmark_filter else None

        env = os.environ.copy()
        env["GOTOOLCHAIN"] = "local"

        if self.progress and version:
            self.progress.set_phase(version, "Collecting benchmarks")
        else:
            print(f"Running benchmarks ({count} iterations, {benchtime} each)...")
            if benchmark_filters:
                for f in benchmark_filters:
                    print(f"  Filter: {f}")

        results = {}
        all_output = []
        all_failed_benchmarks = []  # Track all failed benchmarks across packages

        for pkg in test_packages:
            pkg_path = f"./{pkg}/"

            if self.progress and version:
                if not is_retry:
                    self.progress.update_package(version, pkg, 'running', 0, 0, 0)
                self.progress.log(f"  → Testing {pkg}...")
            else:
                print(f"  → Testing {pkg}...", end=" ", flush=True)

            # Reset benchmark count for this package
            self.streaming_runner.reset_for_package()

            # Determine filter list: run each filter as a separate invocation
            filters_to_run = benchmark_filters if benchmark_filters else [None]

            pkg_output_parts = []
            pkg_failed_benches = []
            pkg_returncode = 0
            pkg_bench_count = 0

            for i, bench_filter in enumerate(filters_to_run):
                # Reset streaming state between filter invocations
                if i > 0:
                    self.streaming_runner.reset_for_package()

                bench_arg = f"-bench={bench_filter}" if bench_filter else "-bench=."

                cmd = [
                    str(go_bin), "test",
                    bench_arg, "-benchmem",
                    f"-count={count}",
                    f"-benchtime={benchtime}",
                    "-timeout=1800s",
                    pkg_path
                ]

                # Run test package with streaming
                returncode, output, failed_benches = self.streaming_runner.run_with_streaming(cmd, env, pkg)

                pkg_output_parts.append(output)
                pkg_failed_benches.extend(failed_benches)
                pkg_bench_count += self.streaming_runner.benchmark_count
                if returncode != 0:
                    pkg_returncode = returncode

            # Combine output from all filter invocations for this package
            output = '\n'.join(pkg_output_parts)
            result = SubprocessResult(returncode=pkg_returncode, stdout=output)

            all_output.append(output)
            all_failed_benchmarks.extend(pkg_failed_benches)

            # Clear the "Running:" line in compact mode
            if not self.progress and not self.verbose:
                print("\r" + " " * 80 + "\r", end="", flush=True)

            # Parse results
            if result.returncode == 0:
                bench_count = pkg_bench_count
                results[pkg] = {
                    'success': True,
                    'error': None,
                    'benchmarks': bench_count,
                    'failed_benchmarks': pkg_failed_benches
                }

                if self.progress and version:
                    needed_retry = len(failed_benches)
                    passed = bench_count - needed_retry
                    if not is_retry:
                        self.progress.update_package(version, pkg, 'done', bench_count, passed, needed_retry)
                    self.progress.log(f"  ✓ {pkg} ({bench_count} benchmarks)")
                else:
                    print(f"✓ ({bench_count} benchmarks)")
            else:
                # Determine failure reason
                error_msg = "Unknown error"
                if "FAIL" in output:
                    # Extract failure line
                    for line in output.split('\n'):
                        if 'FAIL' in line or 'panic' in line or 'fatal error' in line:
                            error_msg = line.strip()
                            break
                elif "timeout" in output.lower():
                    error_msg = "Test timeout"
                elif "build failed" in output.lower():
                    error_msg = "Build failed"

                results[pkg] = {
                    'success': False,
                    'error': error_msg,
                    'benchmarks': pkg_bench_count,
                    'failed_benchmarks': pkg_failed_benches
                }

                if self.progress and version:
                    bench_count = pkg_bench_count
                    needed_retry = len(pkg_failed_benches)
                    passed = bench_count - needed_retry
                    if not is_retry:
                        self.progress.update_package(version, pkg, 'failed', bench_count, passed, needed_retry)
                    self.progress.log(f"  ✗ {pkg} FAILED: {error_msg}")
                else:
                    print(f"✗ FAILED")
                    if self.verbose:
                        print(f"    Error: {error_msg}")

        # Write combined output
        with open(output_file, 'w') as f:
            f.write('\n'.join(all_output))

        # Print summary
        succeeded = [pkg for pkg, res in results.items() if res['success']]
        failed = [pkg for pkg, res in results.items() if not res['success']]

        if not (self.progress and version):
            print()
            print(f"Summary: {len(succeeded)}/{len(test_packages)} packages succeeded")
            if failed:
                print(f"Failed packages: {', '.join(failed)}")
                for pkg in failed:
                    print(f"  {pkg}: {results[pkg]['error']}")

        return {
            'success': len(failed) == 0,
            'results': results,
            'output_file': output_file,
            'failed_packages': failed,
            'failed_benchmarks': all_failed_benchmarks
        }

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



def create_benchmark_filters(benchmark_names: List[str]) -> List[str]:
    """Create Go benchmark filter regexes from list of benchmark names.

    Returns a list of filter strings. When benchmarks are a mix of top-level
    and subtests, splits into two filters so that Go's -bench doesn't skip
    top-level benchmarks (which have no subtests to match a second-level regex).

    Examples:
        ['BenchmarkGCLatency']                    → ['^(BenchmarkGCLatency)$']
        ['BenchmarkRSAKeyGen/Bits4096']           → ['^(BenchmarkRSAKeyGen)$/^(Bits4096)$']
        ['BenchmarkGCLatency',
         'BenchmarkRSAKeyGen/Bits4096']           → ['^(BenchmarkGCLatency)$',
                                                     '^(BenchmarkRSAKeyGen)$/^(Bits4096)$']
    """
    parsed = []
    for name in benchmark_names:
        base = re.sub(r'-\d+$', '', name)  # Remove CPU suffix like -16
        parsed.append(base)

    top_levels = set()
    parents = set()
    subtests = set()
    for name in parsed:
        if '/' in name:
            parent, sub = name.split('/', 1)
            parents.add(parent)
            subtests.add(sub)
        else:
            top_levels.add(name)

    filters = []
    if top_levels:
        filters.append("^(" + "|".join(sorted(top_levels)) + ")$")
    if parents:
        filters.append("^(" + "|".join(sorted(parents)) + ")$/^(" + "|".join(sorted(subtests)) + ")$")
    return filters


def derive_original_output_file(failed_benchmarks_file: Path) -> Path:
    """
    Derive original output file from failed_benchmarks.txt filename.

    Example:
        Input:  results/stable/go1.23/2026-01-26_21-55-10_failed_benchmarks.txt
        Output: results/stable/go1.23/2026-01-26_21-55-10.txt
    """
    stem = failed_benchmarks_file.stem
    if stem.endswith('_failed_benchmarks'):
        # Remove the suffix and reconstruct path
        original_stem = stem[:-len('_failed_benchmarks')]
        return failed_benchmarks_file.parent / f"{original_stem}.txt"
    raise ValueError(f"Invalid failed benchmarks filename: {failed_benchmarks_file.name} (expected format: *_failed_benchmarks.txt)")


def parse_benchmark_file(filepath: Path) -> BenchmarkFile:
    """Parse benchmark output file into structured sections."""
    sections = []
    current_section = None
    header_lines = []
    benchmark_lines_dict = {}  # Temp dict for tracking
    benchmark_order = []  # Track order of first appearance
    footer_lines = []

    with open(filepath, 'r') as f:
        for line in f:
            stripped = line.rstrip('\n')

            # Detect package header (starts with goos:)
            if stripped.startswith('goos:'):
                # Save previous section if exists
                if current_section is not None or header_lines:
                    # Convert dict to ordered list of tuples
                    benchmark_list = [(name, benchmark_lines_dict[name]) for name in benchmark_order]
                    sections.append(PackageSection(
                        header_lines=header_lines,
                        benchmark_lines=benchmark_list,
                        footer_lines=footer_lines
                    ))
                    header_lines = []
                    benchmark_lines_dict = {}
                    benchmark_order = []
                    footer_lines = []

                # Start new section
                header_lines.append(stripped)
                current_section = 'header'

            # Continue header (goarch, pkg, cpu)
            elif current_section == 'header' and (
                stripped.startswith('goarch:') or
                stripped.startswith('pkg:') or
                stripped.startswith('cpu:')
            ):
                header_lines.append(stripped)

            # Benchmark result line
            elif stripped.startswith('Benchmark'):
                current_section = 'benchmarks'
                # Extract benchmark name (before -N suffix)
                match = re.match(r'^(Benchmark\S+)-\d+', stripped)
                if match:
                    bench_name = match.group(1)
                    if bench_name not in benchmark_lines_dict:
                        benchmark_lines_dict[bench_name] = []
                        benchmark_order.append(bench_name)
                    benchmark_lines_dict[bench_name].append(stripped)

            # Footer lines (PASS, ok, FAIL)
            elif current_section == 'benchmarks' and (
                stripped.startswith('PASS') or
                stripped.startswith('ok') or
                stripped.startswith('FAIL')
            ):
                footer_lines.append(stripped)
                current_section = 'footer'

            elif current_section == 'footer':
                footer_lines.append(stripped)

    # Save final section
    if current_section is not None or header_lines:
        benchmark_list = [(name, benchmark_lines_dict[name]) for name in benchmark_order]
        sections.append(PackageSection(
            header_lines=header_lines,
            benchmark_lines=benchmark_list,
            footer_lines=footer_lines
        ))

    return BenchmarkFile(sections=sections)


def merge_benchmark_results(
    original_file: Path,
    retry_file: Path,
    output_file: Path,
    successful_benchmarks: List[str]
) -> bool:
    """
    Merge successful retry results into original file.

    Algorithm:
    1. Parse original file → PackageSection list
    2. Parse retry file → PackageSection list
    3. For each successful benchmark:
       - Find all result lines in retry file
       - Replace corresponding lines in original file
    4. Write merged result atomically (temp + rename)
    5. Preserve all headers, footers, ordering

    Returns: True if successful
    """
    import tempfile

    try:
        # Parse both files
        original = parse_benchmark_file(original_file)
        retry = parse_benchmark_file(retry_file)

        # Build lookup for retry results
        retry_benchmarks = {}
        for section in retry.sections:
            for bench_name, lines in section.benchmark_lines:
                retry_benchmarks[bench_name] = lines

        # Track which benchmarks were successfully merged
        merged_count = 0
        missing_benchmarks = []

        # Replace successful benchmarks in original (preserving order)
        for section in original.sections:
            new_benchmark_lines = []
            for bench_name, lines in section.benchmark_lines:
                if bench_name in successful_benchmarks:
                    if bench_name in retry_benchmarks:
                        # Replace with retry results
                        new_benchmark_lines.append((bench_name, retry_benchmarks[bench_name]))
                        merged_count += 1
                    else:
                        # Keep original if retry doesn't have it
                        new_benchmark_lines.append((bench_name, lines))
                        missing_benchmarks.append(bench_name)
                else:
                    # Keep original for non-successful benchmarks
                    new_benchmark_lines.append((bench_name, lines))

            section.benchmark_lines = new_benchmark_lines

        # Warn if some benchmarks weren't found in retry
        if missing_benchmarks:
            print(f"  ⚠ Warning: {len(missing_benchmarks)} benchmark(s) not found in retry file:", file=sys.stderr)
            for bench in missing_benchmarks[:5]:  # Show first 5
                print(f"    - {bench}", file=sys.stderr)
            if len(missing_benchmarks) > 5:
                print(f"    ... and {len(missing_benchmarks) - 5} more", file=sys.stderr)

        # Write merged result to temp file
        temp_fd, temp_path = tempfile.mkstemp(dir=output_file.parent, suffix='.txt')
        try:
            with os.fdopen(temp_fd, 'w') as f:
                for section in original.sections:
                    # Write headers
                    for line in section.header_lines:
                        f.write(line + '\n')

                    # Write benchmarks (preserve order)
                    for bench_name, lines in section.benchmark_lines:
                        for line in lines:
                            f.write(line + '\n')

                    # Write footers
                    for line in section.footer_lines:
                        f.write(line + '\n')

            # Atomic rename
            shutil.move(temp_path, output_file)

            if merged_count > 0:
                print(f"  ✓ Successfully merged {merged_count} benchmark(s)")

            return True

        except Exception:
            # Clean up temp file on error
            if Path(temp_path).exists():
                os.unlink(temp_path)
            raise

    except Exception as e:
        print(f"✗ Merge failed: {e}", file=sys.stderr)
        import traceback
        traceback.print_exc()
        return False


def run_variance_aware_benchmarks(
    runner: BenchmarkRunner,
    go_bin: Path,
    output_dir: Path,
    timestamp: str,
    benchmark_filters: Optional[List[str]],
    initial_count: int,
    benchtime: str,
    variance_threshold: float,
    rerun_count: int,
    max_reruns: int,
    version: str,
    progress: Optional[ProgressTracker]
) -> Dict[str, any]:
    """
    Run benchmarks with automatic variance checking and retry logic.

    Workflow:
    1. Run benchmarks with initial_count
    2. Analyze variance → identify failed (CV > threshold)
    3. While failed and retry_count < max_reruns:
       - Create benchmark filter
       - Run failed benchmarks with rerun_count
       - Analyze variance
       - Update failed list
    4. Return comprehensive results

    Returns:
        {
            'success': bool,
            'output_file': Path,
            'stats': Dict[str, BenchmarkStats],
            'failed_benchmarks': List[str],
            'retry_count': int,
            'results': dict
        }
    """
    # Run benchmarks
    output_file = output_dir / f"{timestamp}.txt"

    result = runner.run_benchmarks(
        go_bin,
        output_file,
        count=initial_count,
        benchtime=benchtime,
        benchmark_filters=benchmark_filters,
        version=version
    )

    if not result['success']:
        return {
            'success': False,
            'output_file': output_file,
            'stats': {},
            'failed_benchmarks': result.get('failed_benchmarks', []),
            'retry_count': 0,
            'results': result
        }

    print(f"\n✓ Collection complete: {output_file}")

    # Analyze variance
    stats, failed = runner.analyze_variance(output_file, variance_threshold)

    # Auto-retry if failures and retries enabled
    retry_count = 0
    original_output_file = output_file
    while failed and retry_count < max_reruns:
        retry_count += 1
        print(f"\n--- Retry {retry_count}/{max_reruns} ---")
        print(f"Re-running {len(failed)} high-variance benchmark(s) with {rerun_count} iterations...")

        bench_filters = create_benchmark_filters(failed)
        rerun_file = output_dir / f"{timestamp}_retry{retry_count}.txt"

        retry_result = runner.run_benchmarks(
            go_bin,
            rerun_file,
            count=rerun_count,
            benchtime=benchtime,
            benchmark_filters=bench_filters,
            version=version,
            is_retry=True  # Don't overwrite package stats during retries
        )

        if not retry_result['success']:
            print("✗ Retry failed", file=sys.stderr)
            break

        # Analyze retry results
        print("\nRetry variance analysis:")
        retry_stats, retry_failed = runner.analyze_variance(rerun_file, variance_threshold)

        # Use original file for final results (retries are informational)
        output_file = original_output_file
        failed = retry_failed

    # Determine success: True if no failed benchmarks remain after retries
    # This allows --rerun-failed to work with the failed_benchmarks.txt file
    success = len(failed) == 0

    return {
        'success': success,
        'output_file': output_file,
        'stats': stats,
        'failed_benchmarks': failed,
        'retry_count': retry_count,
        'results': result
    }


def main():
    parser = argparse.ArgumentParser(
        description="Go benchmark collection with real-time progress and automatic variance retry",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Basic collection with progress tracking
  %(prog)s 1.23 --count 25 --progress

  # Collection with custom variance threshold and max reruns
  %(prog)s 1.24 --count 25 --progress --variance-threshold 10 --max-reruns 3

  # Re-run only failed benchmarks from a previous collection
  %(prog)s 1.23 --rerun-failed results/stable/go1.23/2026-01-26_14-58-54_failed_benchmarks.txt

  # Multiple versions (always sequential)
  %(prog)s 1.23 1.24 1.25 --count 20 --progress

  # CI/CD mode (skip system checks, no interactive prompts)
  %(prog)s 1.23 --count 25 --progress --skip-system-check
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
        help="Maximum number of re-run attempts for high-variance benchmarks (default: 2)"
    )

    parser.add_argument(
        "--rerun-failed",
        type=Path,
        help="Re-run failed benchmarks from file (e.g., *_failed_benchmarks.txt)"
    )

    parser.add_argument(
        "--verbose",
        action="store_true",
        help="Show detailed benchmark output"
    )

    parser.add_argument(
        "--progress",
        action="store_true",
        help="Enable progress tracking with timestamps (writes to results/collection_progress.json)"
    )

    parser.add_argument(
        "--skip-system-check",
        action="store_true",
        help="Skip system stability checks (useful for CI/CD environments)"
    )

    args = parser.parse_args()

    # Setup paths
    script_dir = Path(__file__).parent.resolve()

    # Initialize progress tracking
    progress = None
    if args.progress:
        progress_file = script_dir.parent / "results" / "collection_progress.json"
        progress_file.parent.mkdir(parents=True, exist_ok=True)
        progress = ProgressTracker(progress_file, args.versions)
        progress.log("="*60)
        progress.log(f"Starting collection for {len(args.versions)} version(s): {', '.join(args.versions)}")
        progress.log("="*60)

    runner = BenchmarkRunner(script_dir, verbose=args.verbose, progress=progress,
                             variance_threshold=args.variance_threshold)

    # Process each version
    for version in args.versions:
        if progress:
            progress.start_version(version)
        else:
            print(f"\n{'='*60}")
            print(f"Go {version} Benchmark Collection")
            print('='*60)

        # Find Go binary
        go_bin = runner.find_go_binary(version)
        if not go_bin:
            print(f"✗ Error: Go {version} not found", file=sys.stderr)
            print(f"  Install with: ./tools/setup-go-versions.sh install <version>")
            continue

        # Detect platform on first binary found
        if runner.results_dir is None:
            platform = runner.detect_platform(go_bin)
            print(f"Platform: {platform}")

        print(f"Go binary: {go_bin}")

        # Create output directory
        output_dir = runner.results_dir / f"go{version}"
        output_dir.mkdir(parents=True, exist_ok=True)

        # Handle rerun mode
        if args.rerun_failed:
            print(f"\n=== Re-run Failed Benchmarks ===")
            print(f"Reading from: {args.rerun_failed}")

            if not args.rerun_failed.exists():
                print(f"✗ File not found: {args.rerun_failed}", file=sys.stderr)
                continue

            # Read failed benchmarks
            with open(args.rerun_failed, 'r') as f:
                failed_benchmarks = [line.strip() for line in f if line.strip()]

            if not failed_benchmarks:
                print("✓ No benchmarks to re-run")
                continue

            print(f"Found {len(failed_benchmarks)} benchmark(s) to re-run")

            # Prepare dependencies
            print(f"\nPreparing dependencies for Go {version}...")
            if not runner.prepare_dependencies(version):
                print(f"⚠ Failed to prepare dependencies for Go {version}, using existing go.mod")

            # Derive original output file and convert to absolute path
            # (needed because we change directories during benchmark execution)
            failed_file = Path(args.rerun_failed).resolve()
            try:
                original_file = derive_original_output_file(args.rerun_failed).resolve()
                print(f"Original file: {original_file}")

                if not original_file.exists():
                    print(f"✗ Original file not found: {original_file}", file=sys.stderr)
                    continue
            except ValueError as e:
                print(f"✗ {e}", file=sys.stderr)
                continue

            # Create benchmark filters (may return multiple for mixed subtest lists)
            bench_filters = create_benchmark_filters(failed_benchmarks)

            # Use same workflow as main collection with _rerun suffix
            # This prevents benchexport from picking up these partial result files
            timestamp = datetime.now().strftime("%Y-%m-%d_%H-%M-%S") + "_rerun"

            result = run_variance_aware_benchmarks(
                runner, go_bin, output_dir, timestamp,
                benchmark_filters=bench_filters,
                initial_count=args.rerun_count,  # Use higher count for reruns
                benchtime=args.benchtime,
                variance_threshold=args.variance_threshold,
                rerun_count=args.rerun_count,
                max_reruns=args.max_reruns,
                version=version,
                progress=progress
            )

            # Determine which benchmarks succeeded:
            # Must be both collected (present in stats) AND passed variance
            successful = [b for b in failed_benchmarks
                          if b in result['stats'] and b not in result['failed_benchmarks']]

            # Warn about benchmarks that weren't collected at all
            not_found = [b for b in failed_benchmarks if b not in result['stats']]
            if not_found:
                print(f"\n⚠ {len(not_found)} benchmark(s) not collected (check filter):")
                for b in not_found:
                    print(f"  - {b}")

            # Merge successful results into original file
            if successful:
                backup_file = original_file.with_suffix('.txt.backup')
                shutil.copy(original_file, backup_file)

                if merge_benchmark_results(original_file, result['output_file'],
                                            original_file, successful):
                    print(f"\n✓ Merged {len(successful)} benchmark(s) into {original_file}")
                    print(f"  Backup: {backup_file}")
                else:
                    print("✗ Merge failed, original preserved")
                    shutil.copy(backup_file, original_file)

            # Report any still-failing benchmarks and update the failed file
            still_failing = [b for b in failed_benchmarks if b in result['failed_benchmarks']]
            with open(failed_file, 'w') as f:
                for b in still_failing:
                    f.write(f"{b}\n")

            if still_failing:
                print(f"\n⚠ {len(still_failing)} still failing after {result['retry_count']} retries:")
                for bench in still_failing:
                    print(f"  - {bench}")
                print(f"  Updated: {failed_file}")
            else:
                print(f"\n✓ All benchmarks passed variance threshold (<{args.variance_threshold}%)")
                print(f"  Cleared: {failed_file}")

            # Update progress with final state
            if progress:
                total = len(result['stats']) if result['stats'] else 0
                progress.complete_version(version, total, success=len(still_failing) == 0)

            continue

        # Normal collection mode
        try:
            # System checks
            if not runner.run_system_check(skip=args.skip_system_check):
                print("Skipping due to system check failure")
                continue

            # Prepare version-specific dependencies
            print(f"\nPreparing dependencies for Go {version}...")
            if not runner.prepare_dependencies(version):
                print(f"⚠ Failed to prepare dependencies for Go {version}, using existing go.mod")

            # Warmup
            if not runner.warmup(go_bin, skip=False, version=version):
                print("Warmup failed, continuing anyway...")

            # Run benchmarks
            timestamp = datetime.now().strftime("%Y-%m-%d_%H-%M-%S")

            result = run_variance_aware_benchmarks(
                runner, go_bin, output_dir, timestamp,
                benchmark_filters=None,  # No filter for main collection
                initial_count=args.count,
                benchtime=args.benchtime,
                variance_threshold=args.variance_threshold,
                rerun_count=args.rerun_count,
                max_reruns=args.max_reruns,
                version=version,
                progress=progress
            )

            if not result['success']:
                print(f"\n✗ Benchmark collection failed", file=sys.stderr)
                print(f"Failed packages: {', '.join(result['results'].get('failed_packages', []))}")

                # Save failed benchmarks info
                if result['failed_benchmarks']:
                    failed_benchmarks_file = output_dir / f"{timestamp}_failed_benchmarks.txt"
                    with open(failed_benchmarks_file, 'w') as f:
                        for bench in result['failed_benchmarks']:
                            f.write(f"{bench}\n")
                    print(f"Failed benchmarks: {len(result['failed_benchmarks'])}")
                    print(f"  Saved to: {failed_benchmarks_file}")

                # Save failed packages info
                if result['results'].get('failed_packages'):
                    failed_packages_file = output_dir / f"{timestamp}_failed_packages.txt"
                    with open(failed_packages_file, 'w') as f:
                        for pkg in result['results']['failed_packages']:
                            f.write(f"{pkg}\n")
                    print(f"Failed packages: {', '.join(result['results']['failed_packages'])}")
                    print(f"  Saved to: {failed_packages_file}")

                # Mark version as complete with failure
                if progress:
                    total = len(result['stats']) if result.get('stats') else 0
                    progress.complete_version(version, total, success=False)

                continue

            # Report final status
            if result['failed_benchmarks']:
                print(f"\n⚠ Warning: {len(result['failed_benchmarks'])} benchmark(s) still have high variance after {result['retry_count']} retries")
                print("  Manual investigation recommended")
            else:
                print(f"\n✓ All benchmarks passed variance threshold (<{args.variance_threshold}%)")

            # Mark version as complete
            if progress:
                total_benchmarks = len(result['stats']) if result['stats'] else 0
                progress.complete_version(version, total_benchmarks, success=True)

        except KeyboardInterrupt:
            # User interrupted
            if progress:
                pkgs = progress.state['versions'][version].get('packages', {})
                total = sum(p.get('benchmarks', 0) for p in pkgs.values())
                progress.complete_version(version, total, success=False)
            print(f"\n✗ Collection interrupted for Go {version}", file=sys.stderr)
            raise  # Re-raise to exit

        except Exception as e:
            # Error occurred
            if progress:
                pkgs = progress.state['versions'][version].get('packages', {})
                total = sum(p.get('benchmarks', 0) for p in pkgs.values())
                progress.complete_version(version, total, success=False)
            print(f"\n✗ Error during collection for Go {version}: {e}", file=sys.stderr)
            continue  # Continue to next version

    # Final summary
    if progress:
        progress.log("="*60)
        progress.log(f"Collection complete - {progress.get_summary()}")
        progress.log("="*60)
        print(f"\nProgress file: {progress.progress_file}")
    else:
        print(f"\n{'='*60}")
        print("Collection complete")
        print('='*60)


if __name__ == "__main__":
    main()
