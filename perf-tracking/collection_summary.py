#!/usr/bin/env python3
"""Generate summary of overnight benchmark collection."""

import sys
from pathlib import Path
from datetime import datetime

sys.path.insert(0, str(Path(__file__).parent / "tools"))
from collect_benchmarks import BenchmarkParser


def summarize_version(version: str, results_dir: Path):
    """Summarize collection results for a Go version."""
    version_dir = results_dir / f"go{version}"

    if not version_dir.exists():
        print(f"  ❌ No results directory found")
        return None

    # Find today's result files
    today = datetime.now().strftime("%Y-%m-%d")
    result_files = sorted(
        [f for f in version_dir.glob(f"{today}*.txt")],
        key=lambda x: x.stat().st_mtime,
        reverse=True
    )

    if not result_files:
        print(f"  ❌ No results from today")
        return None

    # Find final result (merged or latest)
    final_file = None
    for f in result_files:
        if "_merged" in f.name:
            final_file = f
            break

    if not final_file:
        final_file = result_files[0]

    # Parse and analyze
    parser = BenchmarkParser()
    try:
        results = parser.parse_file(final_file)
        stats = parser.calculate_stats(results)

        # Count variance categories
        categories = {'good': 0, 'acceptable': 0, 'warning': 0, 'high': 0, 'very_high': 0}
        for s in stats.values():
            categories[s.category] += 1

        # Print summary
        print(f"  ✓ {len(stats)} unique benchmarks collected")
        print(f"  📁 {final_file.name}")
        print(f"  📊 Variance: {categories['good']} good, {categories['acceptable']} acceptable, "
              f"{categories['warning']} warning, {categories['high']} high, {categories['very_high']} very high")

        if categories['high'] > 0 or categories['very_high'] > 0:
            print(f"  ⚠️  {categories['high'] + categories['very_high']} benchmarks need attention")

        return final_file

    except Exception as e:
        print(f"  ❌ Error parsing results: {e}")
        return None


def main():
    perf_dir = Path(__file__).parent
    results_dir = perf_dir / "results" / "stable"

    print("=" * 60)
    print("Benchmark Collection Summary")
    print("=" * 60)
    print()

    final_files = {}
    for version in ["1.23", "1.24", "1.25"]:
        print(f"Go {version}:")
        final_file = summarize_version(version, results_dir)
        if final_file:
            final_files[version] = final_file
        print()

    # Print export command
    if len(final_files) == 3:
        print("=" * 60)
        print("Next Step: Export to JSON")
        print("=" * 60)
        print()
        print("cd tools/benchexport")
        print("go run . \\")
        for version in ["1.23", "1.24", "1.25"]:
            rel_path = final_files[version].relative_to(perf_dir / "tools" / "benchexport")
            print(f"  --go{version} {rel_path} \\")
        print("  --output ../../docs/03-version-tracking/data")
        print()
    else:
        print("⚠️  Not all versions collected successfully")

    # Check for collection log
    log_file = perf_dir / "collection_overnight.log"
    if log_file.exists():
        print(f"📄 Full log: {log_file}")
        print(f"   Size: {log_file.stat().st_size / 1024:.1f} KB")


if __name__ == "__main__":
    main()
