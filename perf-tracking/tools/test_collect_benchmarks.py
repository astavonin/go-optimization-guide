#!/usr/bin/env python3
"""Unit tests for collect_benchmarks.py"""

import sys
import tempfile
from pathlib import Path

# Import the module
sys.path.insert(0, str(Path(__file__).parent))
from collect_benchmarks import (
    BenchmarkParser, BenchmarkResult, VARIANCE_WARNING,
    derive_original_output_file, parse_benchmark_file, merge_benchmark_results,
    PackageSection, BenchmarkFile
)


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


def test_derive_original_output_file():
    """Test deriving original output file from failed_benchmarks file."""
    # Test valid filename
    failed_file = Path("results/stable/go1.23/2026-01-26_21-55-10_failed_benchmarks.txt")
    original = derive_original_output_file(failed_file)

    assert original == Path("results/stable/go1.23/2026-01-26_21-55-10.txt")
    assert original.name == "2026-01-26_21-55-10.txt"

    # Test with different timestamp
    failed_file2 = Path("/tmp/2026-01-27_14-30-45_failed_benchmarks.txt")
    original2 = derive_original_output_file(failed_file2)

    assert original2 == Path("/tmp/2026-01-27_14-30-45.txt")

    # Test invalid filename (should raise ValueError)
    invalid_file = Path("results/some_random_file.txt")
    try:
        derive_original_output_file(invalid_file)
        assert False, "Should have raised ValueError"
    except ValueError as e:
        assert "Invalid failed benchmarks filename" in str(e)

    print("✓ Derive original output file test passed")


def test_parse_benchmark_file():
    """Test parsing benchmark output file into structured sections."""
    # Create test data with multiple packages
    test_data = """goos: linux
goarch: amd64
pkg: github.com/test/runtime
cpu: Intel Core i7
BenchmarkGC-16         1000000              1234.5 ns/op            256 B/op          4 allocs/op
BenchmarkGC-16         1000000              1245.2 ns/op            256 B/op          4 allocs/op
BenchmarkMap-16        5000000               567.3 ns/op            128 B/op          2 allocs/op
PASS
ok      github.com/test/runtime   10.234s
goos: linux
goarch: amd64
pkg: github.com/test/stdlib
cpu: Intel Core i7
BenchmarkStrings-16    2000000               890.1 ns/op             64 B/op          1 allocs/op
PASS
ok      github.com/test/stdlib    5.123s
"""

    with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.txt') as f:
        f.write(test_data)
        temp_path = Path(f.name)

    try:
        # Parse file
        result = parse_benchmark_file(temp_path)

        # Verify structure
        assert isinstance(result, BenchmarkFile)
        assert len(result.sections) == 2, f"Expected 2 sections, got {len(result.sections)}"

        # Check first section
        section1 = result.sections[0]
        assert len(section1.header_lines) == 4  # goos, goarch, pkg, cpu
        assert section1.header_lines[0] == "goos: linux"
        assert "pkg: github.com/test/runtime" in section1.header_lines

        # Check benchmark lines (now list of tuples)
        assert len(section1.benchmark_lines) == 2  # BenchmarkGC, BenchmarkMap
        bench_names = [name for name, _ in section1.benchmark_lines]
        assert "BenchmarkGC" in bench_names
        assert "BenchmarkMap" in bench_names

        # Check that BenchmarkGC has 2 result lines
        gc_lines = [lines for name, lines in section1.benchmark_lines if name == "BenchmarkGC"][0]
        assert len(gc_lines) == 2

        # Check footer
        assert len(section1.footer_lines) == 2  # PASS, ok
        assert section1.footer_lines[0] == "PASS"

        # Check second section
        section2 = result.sections[1]
        assert "pkg: github.com/test/stdlib" in section2.header_lines
        assert len(section2.benchmark_lines) == 1

        print("✓ Parse benchmark file test passed")

    finally:
        temp_path.unlink()


def test_parse_empty_file():
    """Test parsing empty benchmark file."""
    with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.txt') as f:
        f.write("")
        temp_path = Path(f.name)

    try:
        result = parse_benchmark_file(temp_path)
        assert isinstance(result, BenchmarkFile)
        assert len(result.sections) == 0 or len(result.sections[0].benchmark_lines) == 0

        print("✓ Parse empty file test passed")

    finally:
        temp_path.unlink()


def test_merge_benchmark_results():
    """Test merging retry results into original file."""
    # Create original benchmark output
    original_data = """goos: linux
goarch: amd64
pkg: github.com/test/runtime
cpu: Intel Core i7
BenchmarkGC-16         1000000              1234.5 ns/op            256 B/op          4 allocs/op
BenchmarkGC-16         1000000              1500.0 ns/op            256 B/op          4 allocs/op
BenchmarkMap-16        5000000               567.3 ns/op            128 B/op          2 allocs/op
BenchmarkMap-16        5000000               580.0 ns/op            128 B/op          2 allocs/op
PASS
ok      github.com/test/runtime   10.234s
"""

    # Create retry results (BenchmarkGC improved, BenchmarkMap still present)
    retry_data = """goos: linux
goarch: amd64
pkg: github.com/test/runtime
cpu: Intel Core i7
BenchmarkGC-16         1000000              1235.0 ns/op            256 B/op          4 allocs/op
BenchmarkGC-16         1000000              1236.0 ns/op            256 B/op          4 allocs/op
PASS
ok      github.com/test/runtime   5.123s
"""

    # Write files
    with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.txt') as f:
        f.write(original_data)
        original_path = Path(f.name)

    with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.txt') as f:
        f.write(retry_data)
        retry_path = Path(f.name)

    with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.txt') as f:
        output_path = Path(f.name)

    try:
        # Merge (only BenchmarkGC was successful in retry)
        successful_benchmarks = ["BenchmarkGC"]
        result = merge_benchmark_results(original_path, retry_path, output_path, successful_benchmarks)

        assert result is True, "Merge should succeed"
        assert output_path.exists(), "Output file should exist"

        # Parse merged file
        merged = parse_benchmark_file(output_path)
        assert len(merged.sections) == 1

        section = merged.sections[0]

        # Find BenchmarkGC and BenchmarkMap
        gc_lines = None
        map_lines = None
        for name, lines in section.benchmark_lines:
            if name == "BenchmarkGC":
                gc_lines = lines
            elif name == "BenchmarkMap":
                map_lines = lines

        assert gc_lines is not None, "BenchmarkGC should be present"
        assert map_lines is not None, "BenchmarkMap should be present"

        # BenchmarkGC should have retry results (2 lines with new values)
        assert len(gc_lines) == 2, f"Expected 2 GC lines, got {len(gc_lines)}"
        assert "1235.0 ns/op" in gc_lines[0], "Should have retry result"
        assert "1236.0 ns/op" in gc_lines[1], "Should have retry result"

        # BenchmarkMap should have original results (not in successful_benchmarks)
        assert len(map_lines) == 2, f"Expected 2 Map lines, got {len(map_lines)}"
        assert "567.3 ns/op" in map_lines[0], "Should have original result"

        # Check headers and footers preserved
        assert len(section.header_lines) == 4
        assert "goos: linux" in section.header_lines
        assert len(section.footer_lines) >= 1

        print("✓ Merge benchmark results test passed")

    finally:
        original_path.unlink()
        retry_path.unlink()
        if output_path.exists():
            output_path.unlink()


def test_merge_preserves_order():
    """Test that merge preserves benchmark order."""
    # Create original with specific order: A, B, C
    original_data = """goos: linux
goarch: amd64
pkg: test
cpu: test
BenchmarkA-16    1000    100.0 ns/op
BenchmarkB-16    1000    200.0 ns/op
BenchmarkC-16    1000    300.0 ns/op
PASS
ok      test    1.0s
"""

    # Create retry with only B
    retry_data = """goos: linux
goarch: amd64
pkg: test
cpu: test
BenchmarkB-16    1000    250.0 ns/op
PASS
ok      test    1.0s
"""

    with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.txt') as f:
        f.write(original_data)
        original_path = Path(f.name)

    with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.txt') as f:
        f.write(retry_data)
        retry_path = Path(f.name)

    with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.txt') as f:
        output_path = Path(f.name)

    try:
        # Merge with B as successful
        result = merge_benchmark_results(original_path, retry_path, output_path, ["BenchmarkB"])
        assert result is True

        # Read merged file and check order
        with open(output_path, 'r') as f:
            content = f.read()

        # Extract benchmark lines
        lines = [line for line in content.split('\n') if line.startswith('Benchmark')]

        # Should be in order: A, B (updated), C
        assert len(lines) == 3
        assert "BenchmarkA" in lines[0]
        assert "BenchmarkB" in lines[1]
        assert "250.0 ns/op" in lines[1], "BenchmarkB should have updated value"
        assert "BenchmarkC" in lines[2]

        print("✓ Merge preserves order test passed")

    finally:
        original_path.unlink()
        retry_path.unlink()
        if output_path.exists():
            output_path.unlink()


if __name__ == "__main__":
    print("Running collect_benchmarks.py tests...\n")

    try:
        # Original tests
        test_benchmark_parser()
        test_high_variance_detection()
        test_benchmark_filter_creation()

        # New tests for refactored functionality
        test_derive_original_output_file()
        test_parse_benchmark_file()
        test_parse_empty_file()
        test_merge_benchmark_results()
        test_merge_preserves_order()

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
