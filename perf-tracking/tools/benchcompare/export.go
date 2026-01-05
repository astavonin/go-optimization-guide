package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// VersionData represents all benchmarks for a single Go version
type VersionData struct {
	Version   string               `json:"version"`
	Metadata  VersionMetadata      `json:"metadata"`
	Benchmarks map[string]Benchmark `json:"benchmarks"`
}

type VersionMetadata struct {
	GoVersionFull  string         `json:"go_version_full"`
	CollectedAt    string         `json:"collected_at"`
	System         SystemInfo     `json:"system"`
	BenchmarkConfig BenchmarkConfig `json:"benchmark_config"`
}

type SystemInfo struct {
	CPU  string `json:"cpu"`
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

type BenchmarkConfig struct {
	Iterations int    `json:"iterations"`
	Benchtime  string `json:"benchtime"`
}

type Benchmark struct {
	Name            string  `json:"name"`
	NsPerOp         float64 `json:"ns_per_op"`
	NsPerOpStddev   float64 `json:"ns_per_op_stddev"`
	NsPerOpVariance float64 `json:"ns_per_op_variance"`
	BytesPerOp      int64   `json:"bytes_per_op"`
	AllocsPerOp     int64   `json:"allocs_per_op"`
	Iterations      int64   `json:"iterations"`
	Samples         int     `json:"samples"`
	Description     string  `json:"description,omitempty"`
}

// BenchmarkSample represents a single benchmark run
type BenchmarkSample struct {
	NsPerOp     float64
	BytesPerOp  int64
	AllocsPerOp int64
	Iterations  int64
}

// parseBenchmarkFile parses a raw benchmark result file
func parseBenchmarkFile(filename, version string) (*VersionData, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	versionData := &VersionData{
		Version:    version,
		Benchmarks: make(map[string]Benchmark),
	}

	// Collect samples for each benchmark
	samples := make(map[string][]BenchmarkSample)

	scanner := bufio.NewScanner(file)
	var cpu, goos, goarch string

	for scanner.Scan() {
		line := scanner.Text()

		// Parse header metadata
		if strings.HasPrefix(line, "goos:") {
			goos = strings.TrimSpace(strings.TrimPrefix(line, "goos:"))
		} else if strings.HasPrefix(line, "goarch:") {
			goarch = strings.TrimSpace(strings.TrimPrefix(line, "goarch:"))
		} else if strings.HasPrefix(line, "cpu:") {
			cpu = strings.TrimSpace(strings.TrimPrefix(line, "cpu:"))
		} else if strings.HasPrefix(line, "Benchmark") {
			// Parse benchmark result line
			stats, err := parseBenchmarkLine(line)
			if err != nil {
				continue
			}

			// Store sample
			samples[stats.Name] = append(samples[stats.Name], BenchmarkSample{
				NsPerOp:     stats.NsPerOp,
				BytesPerOp:  stats.BytesPerOp,
				AllocsPerOp: stats.AllocsPerOp,
				Iterations:  1, // We don't track iterations per sample
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	// Calculate statistics for each benchmark
	for name, sampleList := range samples {
		if len(sampleList) == 0 {
			continue
		}

		// Calculate mean
		var sumNs float64
		for _, s := range sampleList {
			sumNs += s.NsPerOp
		}
		meanNs := sumNs / float64(len(sampleList))

		// Calculate standard deviation
		var sumSqDiff float64
		for _, s := range sampleList {
			diff := s.NsPerOp - meanNs
			sumSqDiff += diff * diff
		}
		variance := sumSqDiff / float64(len(sampleList))
		stddev := math.Sqrt(variance)

		// Coefficient of variation (relative standard deviation)
		cv := 0.0
		if meanNs > 0 {
			cv = stddev / meanNs
		}

		// Use last sample for bytes/allocs (they should be consistent)
		lastSample := sampleList[len(sampleList)-1]

		versionData.Benchmarks[name] = Benchmark{
			Name:            name,
			NsPerOp:         meanNs,
			NsPerOpStddev:   stddev,
			NsPerOpVariance: cv,
			BytesPerOp:      lastSample.BytesPerOp,
			AllocsPerOp:     lastSample.AllocsPerOp,
			Samples:         len(sampleList),
			Description:     getBenchmarkDescription(name),
		}
	}

	// Set metadata
	fileInfo, _ := os.Stat(filename)

	// Note: version will be set by caller, so use it if available, else empty
	goVersionStr := versionData.Version
	if goVersionStr == "" {
		goVersionStr = "unknown"
	}

	versionData.Metadata = VersionMetadata{
		GoVersionFull: fmt.Sprintf("go version go%s %s/%s", goVersionStr, goos, goarch),
		CollectedAt:   fileInfo.ModTime().Format(time.RFC3339),
		System: SystemInfo{
			CPU:  cpu,
			OS:   goos,
			Arch: goarch,
		},
		BenchmarkConfig: BenchmarkConfig{
			Iterations: 20,
			Benchtime:  "3s",
		},
	}

	return versionData, nil
}

// getBenchmarkDescription returns a human-readable description
func getBenchmarkDescription(name string) string {
	descriptions := map[string]string{
		"BenchmarkSmallAllocation": "64-byte allocation performance",
		"BenchmarkLargeAllocation": "1MB allocation performance",
		"BenchmarkMapAllocation":   "Map with 100 entries",
		"BenchmarkSliceAppend":     "Slice growth with 1000 appends",
		"BenchmarkGCPressure":      "GC behavior under allocation pressure",
	}
	return descriptions[name]
}

// exportVersion exports a single version's benchmarks to JSON
func exportVersion(inputFile, version, outputFile string) error {
	fmt.Printf("Exporting Go %s...\n", version)
	fmt.Printf("  Input:  %s\n", inputFile)

	versionData, err := parseBenchmarkFile(inputFile, version)
	if err != nil {
		return fmt.Errorf("failed to parse benchmark file: %w", err)
	}

	// Write JSON
	jsonData, err := json.MarshalIndent(versionData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputFile), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	if err := os.WriteFile(outputFile, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	fmt.Printf("  Output: %s\n", outputFile)
	fmt.Printf("  ✓ Exported %d benchmarks\n\n", len(versionData.Benchmarks))

	return nil
}

// IndexData represents the index.json file
type IndexData struct {
	Versions    []VersionInfo `json:"versions"`
	Benchmarks  []string      `json:"benchmarks"`
	LastUpdated string        `json:"last_updated"`
}

type VersionInfo struct {
	Version     string `json:"version"`
	File        string `json:"file"`
	CollectedAt string `json:"collected_at"`
}

// exportAll exports all versions found in the results directory
func exportAll(resultsDir, outputDir string) error {
	fmt.Println("=== Exporting All Versions ===\n")

	// Find all version directories
	entries, err := os.ReadDir(resultsDir)
	if err != nil {
		return fmt.Errorf("failed to read results directory: %w", err)
	}

	var versions []VersionInfo
	benchmarkNames := make(map[string]bool)

	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "go") {
			continue
		}

		version := strings.TrimPrefix(entry.Name(), "go")
		versionDir := filepath.Join(resultsDir, entry.Name())

		// Find latest benchmark file
		files, err := filepath.Glob(filepath.Join(versionDir, "*.txt"))
		if err != nil || len(files) == 0 {
			continue
		}

		// Sort by modification time, newest first
		sort.Slice(files, func(i, j int) bool {
			iInfo, _ := os.Stat(files[i])
			jInfo, _ := os.Stat(files[j])
			return iInfo.ModTime().After(jInfo.ModTime())
		})

		latestFile := files[0]
		outputFile := filepath.Join(outputDir, fmt.Sprintf("go%s.json", version))

		// Export this version
		if err := exportVersion(latestFile, version, outputFile); err != nil {
			fmt.Printf("  Error: %v\n", err)
			continue
		}

		// Read back to get metadata
		data, _ := os.ReadFile(outputFile)
		var versionData VersionData
		if err := json.Unmarshal(data, &versionData); err == nil {
			versions = append(versions, VersionInfo{
				Version:     version,
				File:        fmt.Sprintf("go%s.json", version),
				CollectedAt: versionData.Metadata.CollectedAt,
			})

			// Collect benchmark names
			for name := range versionData.Benchmarks {
				benchmarkNames[name] = true
			}
		}
	}

	// Generate index.json
	var benchmarks []string
	for name := range benchmarkNames {
		benchmarks = append(benchmarks, name)
	}
	sort.Strings(benchmarks)

	indexData := IndexData{
		Versions:    versions,
		Benchmarks:  benchmarks,
		LastUpdated: time.Now().Format(time.RFC3339),
	}

	indexJSON, err := json.MarshalIndent(indexData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal index JSON: %w", err)
	}

	indexFile := filepath.Join(outputDir, "index.json")
	if err := os.WriteFile(indexFile, indexJSON, 0644); err != nil {
		return fmt.Errorf("failed to write index file: %w", err)
	}

	fmt.Println("=== Export Summary ===")
	fmt.Printf("Versions exported: %d\n", len(versions))
	fmt.Printf("Benchmarks found:  %d\n", len(benchmarks))
	fmt.Printf("Output directory:  %s\n", outputDir)
	fmt.Printf("✓ Export complete!\n")

	return nil
}
