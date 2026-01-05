package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type Metadata struct {
	Timestamp     string `json:"timestamp"`
	GoVersion     string `json:"go_version"`
	GoVersionFull string `json:"go_version_full"`
	CommitSha     string `json:"commit_sha"`
	Runner        struct {
		OS    string `json:"os"`
		Arch  string `json:"arch"`
		Cores int    `json:"cores"`
	} `json:"runner"`
}

type BenchmarkResult struct {
	Metadata   Metadata `json:"metadata"`
	Benchmarks []string `json:"benchmarks"`
}

type BenchmarkStats struct {
	Name        string
	NsPerOp     float64
	BytesPerOp  int64
	AllocsPerOp int64
}

type Comparison struct {
	Benchmark      string  `json:"benchmark"`
	BaselineNs     float64 `json:"baseline_ns"`
	TargetNs       float64 `json:"target_ns"`
	DeltaPercent   float64 `json:"delta_percent"`
	BaselineAllocs int64   `json:"baseline_allocs"`
	TargetAllocs   int64   `json:"target_allocs"`
}

// Parse benchmark line like:
// BenchmarkSmallAllocation-16    	1000000000	         3.000 ns/op	       0 B/op	       0 allocs/op
func parseBenchmarkLine(line string) (*BenchmarkStats, error) {
	line = strings.TrimSpace(line)

	// Match benchmark result line
	re := regexp.MustCompile(`^(Benchmark\w+)(?:-\d+)?\s+\d+\s+([\d.]+)\s+ns/op(?:\s+([\d]+)\s+B/op)?(?:\s+([\d]+)\s+allocs/op)?`)
	matches := re.FindStringSubmatch(line)

	if len(matches) < 3 {
		return nil, fmt.Errorf("invalid benchmark line format")
	}

	nsPerOp, err := strconv.ParseFloat(matches[2], 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ns/op: %w", err)
	}

	stats := &BenchmarkStats{
		Name:    matches[1],
		NsPerOp: nsPerOp,
	}

	if len(matches) > 3 && matches[3] != "" {
		bytes, _ := strconv.ParseInt(matches[3], 10, 64)
		stats.BytesPerOp = bytes
	}

	if len(matches) > 4 && matches[4] != "" {
		allocs, _ := strconv.ParseInt(matches[4], 10, 64)
		stats.AllocsPerOp = allocs
	}

	return stats, nil
}

func extractBenchmarks(benchmarkLines []string) map[string]*BenchmarkStats {
	results := make(map[string]*BenchmarkStats)

	for _, line := range benchmarkLines {
		stats, err := parseBenchmarkLine(line)
		if err != nil {
			continue
		}
		// Keep the last (most recent) result for each benchmark
		results[stats.Name] = stats
	}

	return results
}

func compareResults(baseline, target map[string]*BenchmarkStats) []Comparison {
	var comparisons []Comparison

	for name, baseStats := range baseline {
		targetStats, exists := target[name]
		if !exists {
			continue
		}

		delta := ((targetStats.NsPerOp - baseStats.NsPerOp) / baseStats.NsPerOp) * 100

		comparisons = append(comparisons, Comparison{
			Benchmark:      name,
			BaselineNs:     baseStats.NsPerOp,
			TargetNs:       targetStats.NsPerOp,
			DeltaPercent:   delta,
			BaselineAllocs: baseStats.AllocsPerOp,
			TargetAllocs:   targetStats.AllocsPerOp,
		})
	}

	return comparisons
}

func printComparisons(comparisons []Comparison, baseMetadata, targetMetadata Metadata) {
	fmt.Printf("\n=== Benchmark Comparison ===\n\n")
	fmt.Printf("Baseline: %s (%s)\n", baseMetadata.GoVersion, baseMetadata.GoVersionFull)
	fmt.Printf("Target:   %s (%s)\n\n", targetMetadata.GoVersion, targetMetadata.GoVersionFull)

	fmt.Printf("%-30s %15s %15s %12s\n", "Benchmark", "Baseline", "Target", "Change")
	fmt.Printf("%s\n", strings.Repeat("-", 75))

	for _, c := range comparisons {
		direction := "→"
		if c.DeltaPercent > 1 {
			direction = "↑ slower"
		} else if c.DeltaPercent < -1 {
			direction = "↓ faster"
		}

		fmt.Printf("%-30s %12.2f ns %12.2f ns %+9.1f%% %s\n",
			c.Benchmark, c.BaselineNs, c.TargetNs, c.DeltaPercent, direction)
	}
}

func main() {
	// Comparison mode flags
	baseline := flag.String("baseline", "", "Baseline results JSON file")
	target := flag.String("target", "", "Target results JSON file")
	output := flag.String("output", "", "Output comparison file (JSON)")

	// Export mode flags
	exportMode := flag.Bool("export", false, "Export mode: convert benchmark .txt to web JSON")
	exportAllFlag := flag.Bool("export-all", false, "Export all versions from results directory")
	input := flag.String("input", "", "Input benchmark .txt file (for --export)")
	version := flag.String("version", "", "Go version string (for --export)")
	resultsDir := flag.String("results-dir", "", "Results directory (for --export-all)")
	outputDir := flag.String("output-dir", "", "Output directory (for --export-all)")

	flag.Parse()

	// Export mode
	if *exportAllFlag {
		if *resultsDir == "" || *outputDir == "" {
			fmt.Println("Usage: benchcompare --export-all --results-dir <dir> --output-dir <dir>")
			os.Exit(1)
		}
		if err := exportAll(*resultsDir, *outputDir); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if *exportMode {
		if *input == "" || *version == "" || *output == "" {
			fmt.Println("Usage: benchcompare --export --input <file> --version <ver> --output <file>")
			os.Exit(1)
		}
		if err := exportVersion(*input, *version, *output); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Comparison mode (original behavior)
	if *baseline == "" || *target == "" {
		fmt.Println("Usage:")
		fmt.Println("  Compare:    benchcompare -baseline <file> -target <file> [-output <file>]")
		fmt.Println("  Export one: benchcompare --export --input <file> --version <ver> --output <file>")
		fmt.Println("  Export all: benchcompare --export-all --results-dir <dir> --output-dir <dir>")
		os.Exit(1)
	}

	// Read baseline
	baseData, err := os.ReadFile(*baseline)
	if err != nil {
		fmt.Printf("Error reading baseline: %v\n", err)
		os.Exit(1)
	}

	var baseResult BenchmarkResult
	if err := json.Unmarshal(baseData, &baseResult); err != nil {
		fmt.Printf("Error parsing baseline: %v\n", err)
		os.Exit(1)
	}

	// Read target
	targetData, err := os.ReadFile(*target)
	if err != nil {
		fmt.Printf("Error reading target: %v\n", err)
		os.Exit(1)
	}

	var targetResult BenchmarkResult
	if err := json.Unmarshal(targetData, &targetResult); err != nil {
		fmt.Printf("Error parsing target: %v\n", err)
		os.Exit(1)
	}

	// Extract benchmark statistics
	baseStats := extractBenchmarks(baseResult.Benchmarks)
	targetStats := extractBenchmarks(targetResult.Benchmarks)

	// Compare
	comparisons := compareResults(baseStats, targetStats)

	// Print results
	printComparisons(comparisons, baseResult.Metadata, targetResult.Metadata)

	// Save to file if requested
	if *output != "" {
		outputData := struct {
			Baseline    Metadata     `json:"baseline"`
			Target      Metadata     `json:"target"`
			Comparisons []Comparison `json:"comparisons"`
		}{
			Baseline:    baseResult.Metadata,
			Target:      targetResult.Metadata,
			Comparisons: comparisons,
		}

		jsonData, err := json.MarshalIndent(outputData, "", "  ")
		if err != nil {
			fmt.Printf("Error generating JSON: %v\n", err)
			os.Exit(1)
		}

		// Create output directory if needed
		if err := os.MkdirAll(filepath.Dir(*output), 0755); err != nil {
			fmt.Printf("Error creating output directory: %v\n", err)
			os.Exit(1)
		}

		if err := os.WriteFile(*output, jsonData, 0644); err != nil {
			fmt.Printf("Error writing output: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\nComparison saved to: %s\n", *output)
	}
}
