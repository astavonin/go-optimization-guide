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
	Version    string               `json:"version"`
	Metadata   VersionMetadata      `json:"metadata"`
	Benchmarks map[string]Benchmark `json:"benchmarks"`
}

type VersionMetadata struct {
	GoVersionFull   string          `json:"go_version_full"`
	CollectedAt     string          `json:"collected_at"`
	System          SystemInfo      `json:"system"`
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
	Category        string  `json:"category,omitempty"`
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
			Category:        getBenchmarkCategory(name),
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
	// Extract base benchmark name (remove sub-benchmark path and CPU suffix)
	// e.g., "BenchmarkAESCTR/Size1KB-16" -> "BenchmarkAESCTR"
	baseName := name
	if idx := strings.Index(name, "/"); idx != -1 {
		baseName = name[:idx]
	}
	if idx := strings.LastIndex(baseName, "-"); idx != -1 {
		// Check if the suffix after '-' is a number (CPU count)
		if idx+1 < len(baseName) {
			isNumeric := true
			for i := idx + 1; i < len(baseName); i++ {
				if baseName[i] < '0' || baseName[i] > '9' {
					isNumeric = false
					break
				}
			}
			if isNumeric {
				baseName = baseName[:idx]
			}
		}
	}

	descriptions := map[string]string{
		// Runtime/GC benchmarks
		"BenchmarkSmallAllocation":       "64-byte allocation performance",
		"BenchmarkMapCreation":           "Map creation with initial capacity",
		"BenchmarkSwissMapCreation":      "Swiss map creation (Go 1.24+)",
		"BenchmarkSwissMapLarge":         "Large Swiss map operations (Go 1.24+)",
		"BenchmarkSwissMapPresized":      "Swiss map with presizing comparison (Go 1.24+)",
		"BenchmarkSwissMapIteration":     "Swiss map iteration performance (Go 1.24+)",
		"BenchmarkSmallAllocSpecialized": "Specialized small allocations (32-512 bytes)",
		"BenchmarkSyncMap":               "sync.Map concurrent access patterns",
		"BenchmarkGCThroughput":          "GC throughput with mixed allocation patterns",
		"BenchmarkGCLatency":             "Average GC pause latency",
		"BenchmarkGCLatencyP99":          "99th percentile GC pause latency",
		"BenchmarkSmallObjectScanning":   "GC scanning of small object graphs",
		"BenchmarkMediumObjectScanning":  "GC scanning of medium object graphs",
		"BenchmarkLargeObjectScanning":   "GC scanning of large object graphs",
		"BenchmarkAtomicIncrement":       "Atomic counter increment operations",
		"BenchmarkMutexContention":       "Mutex contention under concurrent load",
		"BenchmarkChannelThroughput":     "Channel send/receive throughput",
		"BenchmarkGCMixedWorkload":       "GC performance with mixed allocation patterns",
		"BenchmarkGCSmallObjects":        "GC performance with many small objects",
		"BenchmarkGoroutineCreate":       "Goroutine creation and initialization",
		"BenchmarkStackGrowth":           "Stack growth and shrinking performance",

		// Standard library benchmarks
		"BenchmarkJSONEncode":       "JSON encoding of structured data",
		"BenchmarkJSONDecode":       "JSON decoding into Go structs",
		"BenchmarkJSONDecodeStream": "Streaming JSON decoder performance",
		"BenchmarkIOReadAll":        "io.ReadAll buffer reading performance",
		"BenchmarkAESCTR":           "AES-CTR mode encryption throughput",
		"BenchmarkAESGCM":           "AES-GCM authenticated encryption throughput",
		"BenchmarkSHA":              "SHA hashing throughput (SHA-1, SHA-256, SHA-512, SHA3)",
		"BenchmarkRSAKeyGen":        "RSA key generation performance",
		"BenchmarkRegexp":           "Regular expression matching and compilation",
		"BenchmarkBufferedIO":       "Buffered I/O reader/writer performance",
		"BenchmarkCRC32":            "CRC32 checksum calculation (IEEE, Castagnoli)",
		"BenchmarkFNVHash":          "FNV-1a hash function performance",
		"BenchmarkBinaryEncode":     "Binary encoding methods (encoding/binary)",
		"BenchmarkStringsJoin":      "strings.Join with multiple strings",

		// Legacy names for backwards compatibility
		"BenchmarkReadAll":          "io.ReadAll with small buffers",
		"BenchmarkReadAllLarge":     "io.ReadAll with large buffers (1MB+)",
		"BenchmarkAESCTREncrypt":    "AES-CTR encryption throughput",
		"BenchmarkSHA1Hash":         "SHA-1 hashing throughput",
		"BenchmarkSHA3Hash":         "SHA-3 hashing throughput",
		"BenchmarkRSAKeyGeneration": "RSA 2048-bit key generation",
		"BenchmarkRegexpMatch":      "Regular expression matching",
		"BenchmarkRegexpCompile":    "Regular expression compilation",

		// Networking benchmarks
		"BenchmarkTCPConnect":     "TCP connection establishment time",
		"BenchmarkTCPKeepAlive":   "TCP keep-alive behavior and configuration",
		"BenchmarkTCPThroughput":  "TCP data transfer throughput",
		"BenchmarkTLSHandshake":   "TLS 1.3 handshake performance",
		"BenchmarkTLSResume":      "TLS session resumption",
		"BenchmarkTLSThroughput":  "TLS encrypted data transfer throughput",
		"BenchmarkHTTP2":          "HTTP/2 request handling (sequential/parallel)",
		"BenchmarkHTTPRequest":    "HTTP/1.1 request latency (GET/POST)",
		"BenchmarkConnectionPool": "Connection pool lifecycle and reuse",

		// Legacy runtime benchmarks for backwards compatibility
		"BenchmarkLargeAllocation": "1MB allocation performance",
		"BenchmarkMapAllocation":   "Map with 100 entries",
		"BenchmarkSliceAppend":     "Slice growth with 1000 appends",
		"BenchmarkGCPressure":      "GC behavior under allocation pressure",
	}

	// Try base name first, then fall back to full name for backwards compatibility
	if desc, ok := descriptions[baseName]; ok {
		return desc
	}
	return descriptions[name]
}

// getBenchmarkCategory maps benchmark names to their category
func getBenchmarkCategory(name string) string {
	// Extract base benchmark name (remove sub-benchmark path and CPU suffix)
	// e.g., "BenchmarkAESCTR/Size1KB-16" -> "BenchmarkAESCTR"
	baseName := name
	if idx := strings.Index(name, "/"); idx != -1 {
		baseName = name[:idx]
	}
	if idx := strings.LastIndex(baseName, "-"); idx != -1 {
		// Check if the suffix after '-' is a number (CPU count)
		if idx+1 < len(baseName) {
			isNumeric := true
			for i := idx + 1; i < len(baseName); i++ {
				if baseName[i] < '0' || baseName[i] > '9' {
					isNumeric = false
					break
				}
			}
			if isNumeric {
				baseName = baseName[:idx]
			}
		}
	}

	// Runtime/GC benchmarks
	runtimeBenchmarks := map[string]bool{
		"BenchmarkSmallAllocation":       true,
		"BenchmarkMapCreation":           true,
		"BenchmarkSwissMapCreation":      true,
		"BenchmarkSwissMapLarge":         true,
		"BenchmarkSwissMapPresized":      true,
		"BenchmarkSwissMapIteration":     true,
		"BenchmarkSmallAllocSpecialized": true,
		"BenchmarkSyncMap":               true,
		"BenchmarkGCThroughput":          true,
		"BenchmarkGCLatency":             true,
		"BenchmarkGCLatencyP99":          true,
		"BenchmarkGCSmallObjects":        true,
		"BenchmarkGCMixedWorkload":       true,
		"BenchmarkSmallObjectScanning":   true,
		"BenchmarkMediumObjectScanning":  true,
		"BenchmarkLargeObjectScanning":   true,
		"BenchmarkAtomicIncrement":       true,
		"BenchmarkMutexContention":       true,
		"BenchmarkChannelThroughput":     true,
		"BenchmarkStackGrowth":           true,
		"BenchmarkGoroutineCreate":       true,
		// Legacy benchmarks (backwards compatibility)
		"BenchmarkLargeAllocation": true,
		"BenchmarkMapAllocation":   true,
		"BenchmarkSliceAppend":     true,
		"BenchmarkGCPressure":      true,
	}

	// Standard library benchmarks
	stdlibBenchmarks := map[string]bool{
		"BenchmarkJSONEncode":       true,
		"BenchmarkJSONDecode":       true,
		"BenchmarkJSONDecodeStream": true,
		"BenchmarkIOReadAll":        true,
		"BenchmarkAESCTR":           true,
		"BenchmarkAESGCM":           true,
		"BenchmarkSHA":              true,
		"BenchmarkRSAKeyGen":        true,
		"BenchmarkRegexp":           true,
		"BenchmarkBufferedIO":       true,
		"BenchmarkCRC32":            true,
		"BenchmarkFNVHash":          true,
		"BenchmarkBinaryEncode":     true,
		"BenchmarkStringsJoin":      true,
		// Legacy names for backwards compatibility
		"BenchmarkReadAll":          true,
		"BenchmarkReadAllLarge":     true,
		"BenchmarkAESCTREncrypt":    true,
		"BenchmarkSHA1Hash":         true,
		"BenchmarkSHA3Hash":         true,
		"BenchmarkRSAKeyGeneration": true,
		"BenchmarkRegexpMatch":      true,
		"BenchmarkRegexpCompile":    true,
	}

	// Networking benchmarks
	networkingBenchmarks := map[string]bool{
		"BenchmarkTCPConnect":    true, // TCP connection benchmarks
		"BenchmarkTCPKeepAlive":  true, // TCP keep-alive benchmarks
		"BenchmarkTCPThroughput": true, // TCP throughput benchmarks
		"BenchmarkTLSHandshake":  true, // TLS handshake benchmarks
		"BenchmarkTLSResume":     true, // TLS session resumption
		"BenchmarkTLSThroughput": true, // TLS throughput benchmarks
		"BenchmarkHTTP2":         true, // HTTP/2 benchmarks
		"BenchmarkHTTPRequest":   true, // HTTP request benchmarks
		"BenchmarkConnectionPool": true, // Connection pool benchmarks
	}

	// Try base name first
	if runtimeBenchmarks[baseName] {
		return "runtime"
	}
	if stdlibBenchmarks[baseName] {
		return "stdlib"
	}
	if networkingBenchmarks[baseName] {
		return "networking"
	}

	// Fall back to full name for backwards compatibility
	if runtimeBenchmarks[name] {
		return "runtime"
	}
	if stdlibBenchmarks[name] {
		return "stdlib"
	}
	if networkingBenchmarks[name] {
		return "networking"
	}

	// Default to uncategorized for backwards compatibility
	return "uncategorized"
}

// getBenchmarkSourceFile maps benchmark names to their source file paths
func getBenchmarkSourceFile(name string) string {
	// Extract base benchmark name (remove sub-benchmark path and CPU suffix)
	baseName := name
	if idx := strings.Index(name, "/"); idx != -1 {
		baseName = name[:idx]
	}
	if idx := strings.LastIndex(baseName, "-"); idx != -1 {
		if idx+1 < len(baseName) {
			isNumeric := true
			for i := idx + 1; i < len(baseName); i++ {
				if baseName[i] < '0' || baseName[i] > '9' {
					isNumeric = false
					break
				}
			}
			if isNumeric {
				baseName = baseName[:idx]
			}
		}
	}

	// Runtime/GC benchmarks
	if strings.HasPrefix(baseName, "BenchmarkGC") ||
		strings.HasPrefix(baseName, "BenchmarkMap") ||
		strings.HasPrefix(baseName, "BenchmarkSwiss") ||
		strings.HasPrefix(baseName, "BenchmarkSmallAlloc") ||
		strings.HasPrefix(baseName, "BenchmarkSync") ||
		strings.HasPrefix(baseName, "BenchmarkMutex") ||
		strings.HasPrefix(baseName, "BenchmarkAtomic") ||
		strings.HasPrefix(baseName, "BenchmarkChannel") ||
		strings.HasPrefix(baseName, "BenchmarkGoroutine") ||
		strings.HasPrefix(baseName, "BenchmarkStack") ||
		strings.HasPrefix(baseName, "BenchmarkSmallObject") ||
		strings.HasPrefix(baseName, "BenchmarkMediumObject") ||
		strings.HasPrefix(baseName, "BenchmarkLargeObject") {
		return "perf-tracking/benchmarks/runtime/gc_test.go"
	}

	// Standard library benchmarks
	if strings.HasPrefix(baseName, "BenchmarkJSON") ||
		strings.HasPrefix(baseName, "BenchmarkIO") ||
		strings.HasPrefix(baseName, "BenchmarkReadAll") ||
		strings.HasPrefix(baseName, "BenchmarkAES") ||
		strings.HasPrefix(baseName, "BenchmarkSHA") ||
		strings.HasPrefix(baseName, "BenchmarkRSA") ||
		strings.HasPrefix(baseName, "BenchmarkRegexp") ||
		strings.HasPrefix(baseName, "BenchmarkBuffered") ||
		strings.HasPrefix(baseName, "BenchmarkCRC") ||
		strings.HasPrefix(baseName, "BenchmarkFNV") ||
		strings.HasPrefix(baseName, "BenchmarkBinary") ||
		strings.HasPrefix(baseName, "BenchmarkStrings") {
		return "perf-tracking/benchmarks/stdlib/stdlib_test.go"
	}

	// Networking benchmarks
	if strings.HasPrefix(baseName, "BenchmarkTCP") ||
		strings.HasPrefix(baseName, "BenchmarkTLS") ||
		strings.HasPrefix(baseName, "BenchmarkHTTP") ||
		strings.HasPrefix(baseName, "BenchmarkConnection") {
		return "perf-tracking/benchmarks/networking/networking_test.go"
	}

	// Legacy/unknown
	return "perf-tracking/benchmarks/core/allocation_test.go"
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
	Versions    []VersionInfo   `json:"versions"`
	Benchmarks  []BenchmarkInfo `json:"benchmarks"`
	Repository  RepositoryInfo  `json:"repository"`
	LastUpdated string          `json:"last_updated"`
}

type RepositoryInfo struct {
	URL        string `json:"url"`
	SourcePath string `json:"source_path"`
}

type VersionInfo struct {
	Version     string `json:"version"`
	File        string `json:"file"`
	CollectedAt string `json:"collected_at"`
}

type BenchmarkInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	SourceFile  string `json:"source_file"`
	Category    string `json:"category"`
}

// exportAll exports all versions found in the results directory
func exportAll(resultsDir, outputDir string) error {
	fmt.Println("=== Exporting All Versions ===")

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

		// Find latest benchmark file (excluding retry and failed_benchmarks files)
		files, err := filepath.Glob(filepath.Join(versionDir, "*.txt"))
		if err != nil || len(files) == 0 {
			continue
		}

		// Filter out retry, rerun, failed_benchmarks, and failed_packages files
		var mainFiles []string
		for _, f := range files {
			base := filepath.Base(f)
			if !strings.Contains(base, "_retry") &&
			   !strings.Contains(base, "_rerun") &&
			   !strings.Contains(base, "_failed_benchmarks") &&
			   !strings.Contains(base, "_failed_packages") &&
			   !strings.HasSuffix(base, ".backup") {
				mainFiles = append(mainFiles, f)
			}
		}

		if len(mainFiles) == 0 {
			continue
		}

		// Sort by modification time, newest first
		sort.Slice(mainFiles, func(i, j int) bool {
			iInfo, _ := os.Stat(mainFiles[i])
			jInfo, _ := os.Stat(mainFiles[j])
			return iInfo.ModTime().After(jInfo.ModTime())
		})

		latestFile := mainFiles[0]
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
	var benchmarks []BenchmarkInfo
	for name := range benchmarkNames {
		benchmarks = append(benchmarks, BenchmarkInfo{
			Name:        name,
			Description: getBenchmarkDescription(name),
			SourceFile:  getBenchmarkSourceFile(name),
			Category:    getBenchmarkCategory(name),
		})
	}
	sort.Slice(benchmarks, func(i, j int) bool {
		return benchmarks[i].Name < benchmarks[j].Name
	})

	indexData := IndexData{
		Versions:   versions,
		Benchmarks: benchmarks,
		Repository: RepositoryInfo{
			URL:        "https://github.com/astavonin/go-optimization-guide",
			SourcePath: "blob/main",
		},
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
