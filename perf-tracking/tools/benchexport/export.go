package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
	defer func() { _ = file.Close() }() // read-only; close errors don't affect parsed data

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
		"BenchmarkTCPConnect":     true, // TCP connection benchmarks
		"BenchmarkTCPKeepAlive":   true, // TCP keep-alive benchmarks
		"BenchmarkTCPThroughput":  true, // TCP throughput benchmarks
		"BenchmarkTLSHandshake":   true, // TLS handshake benchmarks
		"BenchmarkTLSResume":      true, // TLS session resumption
		"BenchmarkTLSThroughput":  true, // TLS throughput benchmarks
		"BenchmarkHTTP2":          true, // HTTP/2 benchmarks
		"BenchmarkHTTPRequest":    true, // HTTP request benchmarks
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
	Name        string  `json:"name"`
	Description string  `json:"description"`
	SourceFile  string  `json:"source_file"`
	Category    string  `json:"category"`
	Reliability string  `json:"reliability"` // "reliable", "noisy", or "unstable"
	MaxCV       float64 `json:"max_cv"`       // maximum coefficient of variation observed across all exported versions
}

// PlatformsData represents the top-level platforms.json file
type PlatformsData struct {
	Platforms   []PlatformInfo `json:"platforms"`
	LastUpdated string         `json:"last_updated"`
}

// PlatformInfo describes a single platform entry
type PlatformInfo struct {
	Name    string `json:"name"`
	Display string `json:"display"`
	Index   string `json:"index"`
}

// platformDisplayName returns a human-readable name for a platform string.
func platformDisplayName(platform string) string {
	displayNames := map[string]string{
		"darwin":  "macOS",
		"linux":   "Linux",
		"windows": "Windows",
		"freebsd": "FreeBSD",
	}

	parts := strings.SplitN(platform, "-", 2)
	if len(parts) != 2 {
		return platform
	}

	osName := parts[0]
	arch := parts[1]
	if display, ok := displayNames[osName]; ok {
		osName = display
	}
	return osName + " " + arch
}

// getReliability classifies a benchmark based on its max coefficient of variation
// observed across all exported versions.
//
//	reliable: CV < 5%   — trustworthy for comparison
//	noisy:    5% ≤ CV < 15% — environment-sensitive
//	unstable: CV ≥ 15%  — high variance, not suitable for direct comparison
func getReliability(maxCV float64) string {
	switch {
	case maxCV >= 0.15:
		return "unstable"
	case maxCV >= 0.05:
		return "noisy"
	default:
		return "reliable"
	}
}

// exportAll exports all versions found in the results directory, then rebuilds
// the index from all go*.json files present in the output platform directory.
// This makes every export additive: pre-existing version files are never dropped.
// defaultPlatform is used when the platform cannot be auto-detected from the
// benchmark files (e.g. files lack OS/arch metadata).
func exportAll(resultsDir, outputDir, defaultPlatform string) error {
	fmt.Println("=== Exporting All Versions ===")

	entries, err := os.ReadDir(resultsDir)
	if err != nil {
		return fmt.Errorf("failed to read results directory: %w", err)
	}

	var exportedVersions []string
	var platform string

	// Phase 1: export each go*/ dir found in resultsDir.
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "go") {
			continue
		}

		version := strings.TrimPrefix(entry.Name(), "go")
		versionDir := filepath.Join(resultsDir, entry.Name())

		// Find benchmark files, excluding auxiliary files.
		files, err := filepath.Glob(filepath.Join(versionDir, "*.txt"))
		if err != nil || len(files) == 0 {
			continue
		}

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

		// Sort by modification time, newest first.
		// Pre-cache mtimes so the comparator never calls os.Stat on a file
		// that may have disappeared, which would yield nil and panic.
		mainMtimes := make(map[string]time.Time, len(mainFiles))
		for _, f := range mainFiles {
			if fi, statErr := os.Stat(f); statErr == nil {
				mainMtimes[f] = fi.ModTime()
			}
			// Zero time is a safe fallback; missing files sort last.
		}
		sort.Slice(mainFiles, func(i, j int) bool {
			return mainMtimes[mainFiles[i]].After(mainMtimes[mainFiles[j]])
		})

		latestFile := mainFiles[0]

		// Compute inter-run CV across all main files for this version.
		// This catches benchmarks that appear stable within a single run
		// (low within-run CV) but differ significantly between runs.
		// The resulting CV is written into the exported JSON so that
		// rebuildIndex can use it when computing per-benchmark reliability.
		interRunMaxCV := map[string]float64{}
		if len(mainFiles) > 1 {
			interRunMeans := map[string][]float64{}
			for _, f := range mainFiles {
				fd, err := parseBenchmarkFile(f, version)
				if err != nil {
					continue
				}
				for name, bench := range fd.Benchmarks {
					interRunMeans[name] = append(interRunMeans[name], bench.NsPerOp)
				}
			}
			for name, means := range interRunMeans {
				if len(means) < 2 {
					continue
				}
				mean := 0.0
				for _, m := range means {
					mean += m
				}
				mean /= float64(len(means))
				variance := 0.0
				for _, m := range means {
					variance += (m - mean) * (m - mean)
				}
				interRunMaxCV[name] = math.Sqrt(variance/float64(len(means)-1)) / mean
			}
		}

		// Detect platform from the first available version file.
		if platform == "" {
			probeData, probeErr := parseBenchmarkFile(latestFile, version)
			if probeErr == nil && probeData.Metadata.System.OS != "" && probeData.Metadata.System.Arch != "" {
				platform = probeData.Metadata.System.OS + "-" + probeData.Metadata.System.Arch
			}
		}

		platformDir := filepath.Join(outputDir, platform)
		outputFile := filepath.Join(platformDir, fmt.Sprintf("go%s.json", version))

		if err := exportVersion(latestFile, version, outputFile); err != nil {
			fmt.Printf("  Error: %v\n", err)
			continue
		}

		// Promote inter-run CV into the exported JSON where it exceeds
		// the within-run CV, so rebuildIndex sees the full variance signal.
		if len(interRunMaxCV) > 0 {
			if err := applyInterRunCV(outputFile, interRunMaxCV); err != nil {
				fmt.Printf("  Warning: could not apply inter-run CV: %v\n", err)
			}
		}

		exportedVersions = append(exportedVersions, version)
	}

	if platform == "" {
		platform = defaultPlatform
		fmt.Printf("  Platform not detected from files; using default: %s\n", platform)
	}

	// Phase 2: rebuild index from ALL go*.json files in the platform output
	// directory (both newly written and pre-existing), so no version is lost.
	platformDir := filepath.Join(outputDir, platform)
	if err := rebuildIndex(platformDir, outputDir, platform); err != nil {
		return fmt.Errorf("failed to rebuild index: %w", err)
	}

	// Read back the rebuilt index for accurate summary counts.
	var indexData IndexData
	if data, err := os.ReadFile(filepath.Join(platformDir, "index.json")); err == nil {
		if unmarshalErr := json.Unmarshal(data, &indexData); unmarshalErr != nil {
			fmt.Printf("  Warning: could not parse rebuilt index for summary: %v\n", unmarshalErr)
		}
	}

	exportedStrs := make([]string, len(exportedVersions))
	for i, v := range exportedVersions {
		exportedStrs[i] = "go" + v
	}
	totalStrs := make([]string, len(indexData.Versions))
	for i, v := range indexData.Versions {
		totalStrs[i] = "go" + v.Version
	}

	fmt.Println("=== Export Summary ===")
	fmt.Printf("Platform:          %s\n", platform)
	fmt.Printf("Exported this run: %d (%s)\n", len(exportedVersions), strings.Join(exportedStrs, ", "))
	fmt.Printf("Total in index:    %d (%s)\n", len(indexData.Versions), strings.Join(totalStrs, ", "))
	fmt.Printf("Benchmarks:        %d\n", len(indexData.Benchmarks))
	fmt.Printf("✓ Export complete!\n")

	return nil
}

// applyInterRunCV updates NsPerOpVariance in the exported JSON for any benchmark
// where the inter-run CV exceeds the within-run CV already stored.
func applyInterRunCV(outputFile string, interRunMaxCV map[string]float64) error {
	data, err := os.ReadFile(outputFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", outputFile, err)
	}

	var vd VersionData
	if err := json.Unmarshal(data, &vd); err != nil {
		return fmt.Errorf("failed to unmarshal %s: %w", outputFile, err)
	}

	updated := false
	for name, irCV := range interRunMaxCV {
		if b, ok := vd.Benchmarks[name]; ok && irCV > b.NsPerOpVariance {
			b.NsPerOpVariance = irCV
			vd.Benchmarks[name] = b
			updated = true
		}
	}

	if !updated {
		return nil
	}

	jsonData, err := json.MarshalIndent(vd, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal %s: %w", outputFile, err)
	}
	return os.WriteFile(outputFile, jsonData, 0644)
}

// rebuildIndex scans all go<version>.json files in platformDir, computes
// benchmarkMaxCV across all versions, and writes a complete index.json.
// It also keeps platforms.json current via updatePlatformsJSON.
func rebuildIndex(platformDir, outputDir, platform string) error {
	jsonFiles, err := filepath.Glob(filepath.Join(platformDir, "go*.json"))
	if err != nil {
		return fmt.Errorf("failed to glob json files: %w", err)
	}

	// Keep only files whose name starts with go<digit> (e.g. go1.24.json).
	var validFiles []string
	for _, f := range jsonFiles {
		base := filepath.Base(f)
		if len(base) > 2 && base[2] >= '0' && base[2] <= '9' {
			validFiles = append(validFiles, f)
		}
	}

	// Pre-cache mtimes so the comparator never calls os.Stat on a file that
	// may have disappeared between glob and sort, which would yield nil and panic.
	fileMtimes := make(map[string]time.Time, len(validFiles))
	for _, f := range validFiles {
		if fi, statErr := os.Stat(f); statErr == nil {
			fileMtimes[f] = fi.ModTime()
		}
		// Zero time is a safe fallback; missing files sort last on tie-break.
	}

	// Sort ascending by version number, newest mtime first within the same version.
	// This ensures that when two files share the same JSON version string (e.g.
	// go1.26.json and go1.26.0.json both contain "version":"1.26"), we process
	// the most recently written file first and skip the stale duplicate.
	sort.Slice(validFiles, func(i, j int) bool {
		vi := versionFromJSONFilename(filepath.Base(validFiles[i]))
		vj := versionFromJSONFilename(filepath.Base(validFiles[j]))
		cmp := compareVersionStrings(vi, vj)
		if cmp != 0 {
			return cmp < 0
		}
		// Tie-break: prefer most recently modified file.
		return fileMtimes[validFiles[i]].After(fileMtimes[validFiles[j]])
	})

	var versions []VersionInfo
	benchmarkNames := make(map[string]bool)
	benchmarkMaxCV := map[string]float64{}
	seenVersions := make(map[string]bool)

	for _, f := range validFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			fmt.Printf("  Warning: skipping %s: %v\n", filepath.Base(f), err)
			continue
		}
		var vd VersionData
		if err := json.Unmarshal(data, &vd); err != nil {
			fmt.Printf("  Warning: skipping %s (parse error): %v\n", filepath.Base(f), err)
			continue
		}

		// Skip stale duplicates: keep only the first (newest) file per version.
		if seenVersions[vd.Version] {
			continue
		}
		seenVersions[vd.Version] = true

		versions = append(versions, VersionInfo{
			Version:     vd.Version,
			File:        filepath.Base(f),
			CollectedAt: vd.Metadata.CollectedAt,
		})

		for name, bench := range vd.Benchmarks {
			benchmarkNames[name] = true
			if bench.NsPerOpVariance > benchmarkMaxCV[name] {
				benchmarkMaxCV[name] = bench.NsPerOpVariance
			}
		}
	}

	var benchmarks []BenchmarkInfo
	for name := range benchmarkNames {
		benchmarks = append(benchmarks, BenchmarkInfo{
			Name:        name,
			Description: getBenchmarkDescription(name),
			SourceFile:  getBenchmarkSourceFile(name),
			Category:    getBenchmarkCategory(name),
			Reliability: getReliability(benchmarkMaxCV[name]),
			MaxCV:       benchmarkMaxCV[name],
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

	if err := os.WriteFile(filepath.Join(platformDir, "index.json"), indexJSON, 0644); err != nil {
		return fmt.Errorf("failed to write index file: %w", err)
	}

	return updatePlatformsJSON(outputDir, platform)
}

// versionFromJSONFilename extracts the version string from a filename like "go1.24.json".
func versionFromJSONFilename(filename string) string {
	s := strings.TrimPrefix(filename, "go")
	return strings.TrimSuffix(s, ".json")
}

// compareVersionStrings compares two dot-separated version strings (e.g. "1.23", "1.24.1").
// Returns negative if a < b, 0 if equal, positive if a > b.
// Version parts are expected to be purely numeric; non-numeric components (e.g. "rc1")
// are treated as 0 by strconv.Atoi. Go benchmark filenames use only stable release
// versions so this is safe, but pre-release suffixes would sort incorrectly.
func compareVersionStrings(a, b string) int {
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")
	maxLen := len(partsA)
	if len(partsB) > maxLen {
		maxLen = len(partsB)
	}
	for i := 0; i < maxLen; i++ {
		var va, vb int
		if i < len(partsA) {
			va, _ = strconv.Atoi(partsA[i])
		}
		if i < len(partsB) {
			vb, _ = strconv.Atoi(partsB[i])
		}
		if va != vb {
			if va < vb {
				return -1
			}
			return 1
		}
	}
	return 0
}

// updatePlatformsJSON reads an existing platforms.json (if present), merges/updates
// the current platform entry, and writes back the updated file.
func updatePlatformsJSON(outputDir, platform string) error {
	platformsFile := filepath.Join(outputDir, "platforms.json")

	var platformsData PlatformsData

	// Read existing platforms.json if present
	if data, err := os.ReadFile(platformsFile); err == nil {
		_ = json.Unmarshal(data, &platformsData)
	}

	// Update or add the current platform entry
	newEntry := PlatformInfo{
		Name:    platform,
		Display: platformDisplayName(platform),
		Index:   platform + "/index.json",
	}

	found := false
	for i, p := range platformsData.Platforms {
		if p.Name == platform {
			platformsData.Platforms[i] = newEntry
			found = true
			break
		}
	}
	if !found {
		platformsData.Platforms = append(platformsData.Platforms, newEntry)
	}

	// Sort platforms by name for stable output
	sort.Slice(platformsData.Platforms, func(i, j int) bool {
		return platformsData.Platforms[i].Name < platformsData.Platforms[j].Name
	})

	platformsData.LastUpdated = time.Now().Format(time.RFC3339)

	jsonData, err := json.MarshalIndent(platformsData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal platforms JSON: %w", err)
	}

	if err := os.WriteFile(platformsFile, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write platforms.json: %w", err)
	}

	return nil
}
