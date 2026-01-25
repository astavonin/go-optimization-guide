#!/usr/bin/env python3
"""Unit tests for collect_benchmarks.py"""

import sys
import tempfile
from pathlib import Path

# Import the module
sys.path.insert(0, str(Path(__file__).parent))
from collect_benchmarks import BenchmarkParser, BenchmarkResult, VARIANCE_WARNING


def test_benchmark_parser():
    """Test parsing of benchmark output."""
    # Create test data
    test_data = """goos: linux
goarch: amd64
pkg: github.com/astavonin/go-optimization-guide/benchmarks/runtime
cpu: 13th Gen Intel(R) Core(TM) i5-13450HX
BenchmarkGCThroughput-16         1000000              1234.5 ns/op            256 B/op          4 allocs/op
BenchmarkGCThroughput-16         1000000              1245.2 ns/op            256 B/op          4 allocs/op
BenchmarkGCThroughput-16         1000000              1230.8 ns/op            256 B/op          4 allocs/op
BenchmarkMapAccess-16            5000000               567.3 ns/op            128 B/op          2 allocs/op
BenchmarkMapAccess-16            5000000               570.1 ns/op            128 B/op          2 allocs/op
PASS
ok      github.com/astavonin/go-optimization-guide/benchmarks/runtime   10.234s
"""

    # Write to temp file
    with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.txt') as f:
        f.write(test_data)
        temp_path = Path(f.name)

    try:
        # Parse
        parser = BenchmarkParser()
        results = parser.parse_file(temp_path)

        # Verify results
        assert len(results) == 5, f"Expected 5 results, got {len(results)}"

        # Check first result
        assert results[0].name == "BenchmarkGCThroughput"
        assert results[0].iterations == 1000000
        assert results[0].ns_per_op == 1234.5
        assert results[0].bytes_per_op == 256
        assert results[0].allocs_per_op == 4

        # Check fourth result (different benchmark)
        assert results[3].name == "BenchmarkMapAccess"
        assert results[3].iterations == 5000000
        assert results[3].ns_per_op == 567.3

        print("✓ Benchmark parsing test passed")

        # Test statistics calculation
        stats = parser.calculate_stats(results)

        assert len(stats) == 2, f"Expected 2 unique benchmarks, got {len(stats)}"

        # Check GCThroughput stats
        gc_stats = stats["BenchmarkGCThroughput"]
        assert gc_stats.count == 3, f"Expected 3 samples, got {gc_stats.count}"
        assert abs(gc_stats.mean - 1236.83) < 1, f"Mean calculation error: {gc_stats.mean}"

        # CV should be low (values are close)
        assert gc_stats.cv < 1.0, f"CV too high for stable data: {gc_stats.cv}%"
        assert gc_stats.category == "good"

        # Check MapAccess stats
        map_stats = stats["BenchmarkMapAccess"]
        assert map_stats.count == 2
        assert map_stats.category in ["good", "acceptable"]

        print("✓ Variance calculation test passed")

    finally:
        temp_path.unlink()


def test_high_variance_detection():
    """Test detection of high-variance benchmarks."""
    # Create test data with one high-variance benchmark
    test_data = """goos: linux
goarch: amd64
BenchmarkStable-16      1000000    100.0 ns/op
BenchmarkStable-16      1000000    101.0 ns/op
BenchmarkStable-16      1000000    99.5 ns/op
BenchmarkUnstable-16    1000000    100.0 ns/op
BenchmarkUnstable-16    1000000    150.0 ns/op
BenchmarkUnstable-16    1000000    200.0 ns/op
"""

    with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.txt') as f:
        f.write(test_data)
        temp_path = Path(f.name)

    try:
        parser = BenchmarkParser()
        results = parser.parse_file(temp_path)
        stats = parser.calculate_stats(results)

        # Stable benchmark should pass
        stable = stats["BenchmarkStable"]
        assert stable.passed, f"Stable benchmark failed: CV={stable.cv}%"
        assert stable.category == "good"

        # Unstable benchmark should fail
        unstable = stats["BenchmarkUnstable"]
        assert not unstable.passed, f"Unstable benchmark passed: CV={unstable.cv}%"
        assert unstable.cv > VARIANCE_WARNING
        assert unstable.category in ["high", "very_high"]

        print("✓ High variance detection test passed")

    finally:
        temp_path.unlink()


def test_benchmark_filter_creation():
    """Test creation of Go benchmark filter regex."""
    from collect_benchmarks import create_benchmark_filter

    # Test with simple names
    names = ["BenchmarkGCThroughput", "BenchmarkMapAccess"]
    filter_regex = create_benchmark_filter(names)
    assert filter_regex == "^(BenchmarkGCThroughput|BenchmarkMapAccess)$"

    # Test with CPU suffix
    names_with_cpu = ["BenchmarkGCThroughput-16", "BenchmarkMapAccess-16"]
    filter_regex = create_benchmark_filter(names_with_cpu)
    assert filter_regex == "^(BenchmarkGCThroughput|BenchmarkMapAccess)$"

    print("✓ Benchmark filter creation test passed")


if __name__ == "__main__":
    print("Running collect_benchmarks.py tests...\n")

    try:
        test_benchmark_parser()
        test_high_variance_detection()
        test_benchmark_filter_creation()

        print("\n" + "="*60)
        print("All tests passed! ✓")
        print("="*60)

    except AssertionError as e:
        print(f"\n✗ Test failed: {e}", file=sys.stderr)
        sys.exit(1)
    except Exception as e:
        print(f"\n✗ Unexpected error: {e}", file=sys.stderr)
        import traceback
        traceback.print_exc()
        sys.exit(1)
