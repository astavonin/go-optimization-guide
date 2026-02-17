package main

import (
	"encoding/json"
	"os"
	"testing"
)

func TestGetBenchmarkCategory(t *testing.T) {
	tests := []struct {
		name          string
		benchmarkName string
		wantCategory  string
	}{
		// Runtime/GC benchmarks
		{
			name:          "Small allocation benchmark",
			benchmarkName: "BenchmarkSmallAllocation",
			wantCategory:  "runtime",
		},
		{
			name:          "Map creation benchmark",
			benchmarkName: "BenchmarkMapCreation",
			wantCategory:  "runtime",
		},
		{
			name:          "Swiss map creation benchmark",
			benchmarkName: "BenchmarkSwissMapCreation",
			wantCategory:  "runtime",
		},
		{
			name:          "Swiss map large benchmark",
			benchmarkName: "BenchmarkSwissMapLarge",
			wantCategory:  "runtime",
		},
		{
			name:          "GC throughput benchmark",
			benchmarkName: "BenchmarkGCThroughput",
			wantCategory:  "runtime",
		},
		{
			name:          "GC latency benchmark",
			benchmarkName: "BenchmarkGCLatency",
			wantCategory:  "runtime",
		},
		{
			name:          "GC latency P99 benchmark",
			benchmarkName: "BenchmarkGCLatencyP99",
			wantCategory:  "runtime",
		},
		{
			name:          "GC small objects benchmark",
			benchmarkName: "BenchmarkGCSmallObjects",
			wantCategory:  "runtime",
		},
		{
			name:          "GC mixed workload benchmark",
			benchmarkName: "BenchmarkGCMixedWorkload",
			wantCategory:  "runtime",
		},
		{
			name:          "Small object scanning benchmark",
			benchmarkName: "BenchmarkSmallObjectScanning",
			wantCategory:  "runtime",
		},
		{
			name:          "Medium object scanning benchmark",
			benchmarkName: "BenchmarkMediumObjectScanning",
			wantCategory:  "runtime",
		},
		{
			name:          "Large object scanning benchmark",
			benchmarkName: "BenchmarkLargeObjectScanning",
			wantCategory:  "runtime",
		},
		{
			name:          "Atomic increment benchmark",
			benchmarkName: "BenchmarkAtomicIncrement",
			wantCategory:  "runtime",
		},
		{
			name:          "Mutex contention benchmark",
			benchmarkName: "BenchmarkMutexContention",
			wantCategory:  "runtime",
		},
		{
			name:          "Channel throughput benchmark",
			benchmarkName: "BenchmarkChannelThroughput",
			wantCategory:  "runtime",
		},
		{
			name:          "Stack growth benchmark",
			benchmarkName: "BenchmarkStackGrowth",
			wantCategory:  "runtime",
		},
		{
			name:          "Goroutine create benchmark",
			benchmarkName: "BenchmarkGoroutineCreate",
			wantCategory:  "runtime",
		},
		{
			name:          "Swiss map presized benchmark",
			benchmarkName: "BenchmarkSwissMapPresized",
			wantCategory:  "runtime",
		},
		{
			name:          "Swiss map presized with sub-benchmark",
			benchmarkName: "BenchmarkSwissMapPresized/Presized-16",
			wantCategory:  "runtime",
		},
		{
			name:          "Swiss map iteration benchmark",
			benchmarkName: "BenchmarkSwissMapIteration",
			wantCategory:  "runtime",
		},
		{
			name:          "Swiss map iteration with sub-benchmark",
			benchmarkName: "BenchmarkSwissMapIteration/Size1000-16",
			wantCategory:  "runtime",
		},
		{
			name:          "Small alloc specialized benchmark",
			benchmarkName: "BenchmarkSmallAllocSpecialized",
			wantCategory:  "runtime",
		},
		{
			name:          "Small alloc specialized with sub-benchmark",
			benchmarkName: "BenchmarkSmallAllocSpecialized/Size64-16",
			wantCategory:  "runtime",
		},
		{
			name:          "Sync map benchmark",
			benchmarkName: "BenchmarkSyncMap",
			wantCategory:  "runtime",
		},
		{
			name:          "Sync map with sub-benchmark",
			benchmarkName: "BenchmarkSyncMap/Parallel-16",
			wantCategory:  "runtime",
		},

		// Standard library benchmarks (actual names from results)
		{
			name:          "JSON encode benchmark",
			benchmarkName: "BenchmarkJSONEncode",
			wantCategory:  "stdlib",
		},
		{
			name:          "JSON decode benchmark",
			benchmarkName: "BenchmarkJSONDecode",
			wantCategory:  "stdlib",
		},
		{
			name:          "JSON encode with sub-benchmark",
			benchmarkName: "BenchmarkJSONEncode/Small-16",
			wantCategory:  "stdlib",
		},
		{
			name:          "JSON decode with sub-benchmark",
			benchmarkName: "BenchmarkJSONDecode/Large-16",
			wantCategory:  "stdlib",
		},
		{
			name:          "IO ReadAll benchmark",
			benchmarkName: "BenchmarkIOReadAll",
			wantCategory:  "stdlib",
		},
		{
			name:          "IO ReadAll with sub-benchmark",
			benchmarkName: "BenchmarkIOReadAll/Size1KB-16",
			wantCategory:  "stdlib",
		},
		{
			name:          "AES CTR benchmark",
			benchmarkName: "BenchmarkAESCTR",
			wantCategory:  "stdlib",
		},
		{
			name:          "AES CTR with sub-benchmark",
			benchmarkName: "BenchmarkAESCTR/Size1KB-16",
			wantCategory:  "stdlib",
		},
		{
			name:          "AES GCM benchmark",
			benchmarkName: "BenchmarkAESGCM",
			wantCategory:  "stdlib",
		},
		{
			name:          "AES GCM with sub-benchmark",
			benchmarkName: "BenchmarkAESGCM/Size16KB-16",
			wantCategory:  "stdlib",
		},
		{
			name:          "SHA benchmark",
			benchmarkName: "BenchmarkSHA",
			wantCategory:  "stdlib",
		},
		{
			name:          "SHA with sub-benchmark",
			benchmarkName: "BenchmarkSHA/SHA256-16",
			wantCategory:  "stdlib",
		},
		{
			name:          "RSA key generation benchmark",
			benchmarkName: "BenchmarkRSAKeyGen",
			wantCategory:  "stdlib",
		},
		{
			name:          "RSA key generation with sub-benchmark",
			benchmarkName: "BenchmarkRSAKeyGen/Bits2048-16",
			wantCategory:  "stdlib",
		},
		{
			name:          "Regexp benchmark",
			benchmarkName: "BenchmarkRegexp",
			wantCategory:  "stdlib",
		},
		{
			name:          "Regexp with sub-benchmark",
			benchmarkName: "BenchmarkRegexp/Match/Email-16",
			wantCategory:  "stdlib",
		},
		{
			name:          "Buffered IO benchmark",
			benchmarkName: "BenchmarkBufferedIO",
			wantCategory:  "stdlib",
		},
		{
			name:          "Buffered IO with sub-benchmark",
			benchmarkName: "BenchmarkBufferedIO/Reader-16",
			wantCategory:  "stdlib",
		},
		{
			name:          "CRC32 benchmark",
			benchmarkName: "BenchmarkCRC32",
			wantCategory:  "stdlib",
		},
		{
			name:          "CRC32 with sub-benchmark",
			benchmarkName: "BenchmarkCRC32/IEEE-16",
			wantCategory:  "stdlib",
		},
		{
			name:          "FNV hash benchmark",
			benchmarkName: "BenchmarkFNVHash",
			wantCategory:  "stdlib",
		},
		{
			name:          "FNV hash with sub-benchmark",
			benchmarkName: "BenchmarkFNVHash/FNV1a_64-16",
			wantCategory:  "stdlib",
		},
		{
			name:          "Binary encode benchmark",
			benchmarkName: "BenchmarkBinaryEncode",
			wantCategory:  "stdlib",
		},
		{
			name:          "Binary encode with sub-benchmark",
			benchmarkName: "BenchmarkBinaryEncode/Encode-16",
			wantCategory:  "stdlib",
		},
		// Legacy names for backwards compatibility
		{
			name:          "ReadAll legacy benchmark",
			benchmarkName: "BenchmarkReadAll",
			wantCategory:  "stdlib",
		},
		{
			name:          "AES CTR encrypt legacy benchmark",
			benchmarkName: "BenchmarkAESCTREncrypt",
			wantCategory:  "stdlib",
		},
		{
			name:          "SHA1 hash legacy benchmark",
			benchmarkName: "BenchmarkSHA1Hash",
			wantCategory:  "stdlib",
		},
		{
			name:          "Regexp match legacy benchmark",
			benchmarkName: "BenchmarkRegexpMatch",
			wantCategory:  "stdlib",
		},

		// Networking benchmarks
		{
			name:          "TCP connect benchmark",
			benchmarkName: "BenchmarkTCPConnect",
			wantCategory:  "networking",
		},
		{
			name:          "TCP keep-alive benchmark",
			benchmarkName: "BenchmarkTCPKeepAlive",
			wantCategory:  "networking",
		},
		{
			name:          "TCP throughput benchmark",
			benchmarkName: "BenchmarkTCPThroughput",
			wantCategory:  "networking",
		},
		{
			name:          "TLS handshake benchmark",
			benchmarkName: "BenchmarkTLSHandshake",
			wantCategory:  "networking",
		},
		{
			name:          "TLS resume benchmark",
			benchmarkName: "BenchmarkTLSResume",
			wantCategory:  "networking",
		},
		{
			name:          "TLS throughput benchmark",
			benchmarkName: "BenchmarkTLSThroughput",
			wantCategory:  "networking",
		},
		{
			name:          "HTTP/2 benchmark",
			benchmarkName: "BenchmarkHTTP2",
			wantCategory:  "networking",
		},
		{
			name:          "HTTP request benchmark",
			benchmarkName: "BenchmarkHTTPRequest",
			wantCategory:  "networking",
		},
		{
			name:          "Connection pool benchmark",
			benchmarkName: "BenchmarkConnectionPool",
			wantCategory:  "networking",
		},

		// Unknown/uncategorized benchmarks
		{
			name:          "Unknown benchmark",
			benchmarkName: "BenchmarkSomethingUnknown",
			wantCategory:  "uncategorized",
		},
		{
			name:          "Legacy benchmark",
			benchmarkName: "BenchmarkCustomOperation",
			wantCategory:  "uncategorized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getBenchmarkCategory(tt.benchmarkName)
			if got != tt.wantCategory {
				t.Errorf("getBenchmarkCategory(%q) = %q, want %q", tt.benchmarkName, got, tt.wantCategory)
			}
		})
	}
}

func TestGetBenchmarkDescription(t *testing.T) {
	tests := []struct {
		name          string
		benchmarkName string
		wantEmpty     bool
		contains      string
	}{
		// Known runtime benchmarks with descriptions
		{
			name:          "Small allocation has description",
			benchmarkName: "BenchmarkSmallAllocation",
			wantEmpty:     false,
			contains:      "64-byte",
		},
		{
			name:          "Map creation has description",
			benchmarkName: "BenchmarkMapCreation",
			wantEmpty:     false,
			contains:      "Map creation",
		},
		{
			name:          "GC throughput has description",
			benchmarkName: "BenchmarkGCThroughput",
			wantEmpty:     false,
			contains:      "GC throughput",
		},
		{
			name:          "GC mixed workload has description",
			benchmarkName: "BenchmarkGCMixedWorkload",
			wantEmpty:     false,
			contains:      "mixed allocation",
		},
		{
			name:          "GC small objects has description",
			benchmarkName: "BenchmarkGCSmallObjects",
			wantEmpty:     false,
			contains:      "small objects",
		},
		{
			name:          "Goroutine create has description",
			benchmarkName: "BenchmarkGoroutineCreate",
			wantEmpty:     false,
			contains:      "Goroutine creation",
		},
		{
			name:          "Stack growth has description",
			benchmarkName: "BenchmarkStackGrowth",
			wantEmpty:     false,
			contains:      "Stack growth",
		},
		{
			name:          "Swiss map large has description",
			benchmarkName: "BenchmarkSwissMapLarge",
			wantEmpty:     false,
			contains:      "Swiss map",
		},
		{
			name:          "Swiss map presized has description",
			benchmarkName: "BenchmarkSwissMapPresized",
			wantEmpty:     false,
			contains:      "presizing",
		},
		{
			name:          "Swiss map iteration has description",
			benchmarkName: "BenchmarkSwissMapIteration",
			wantEmpty:     false,
			contains:      "iteration",
		},
		{
			name:          "Small alloc specialized has description",
			benchmarkName: "BenchmarkSmallAllocSpecialized",
			wantEmpty:     false,
			contains:      "Specialized",
		},
		{
			name:          "Sync map has description",
			benchmarkName: "BenchmarkSyncMap",
			wantEmpty:     false,
			contains:      "sync.Map",
		},
		{
			name:          "Sub-benchmark extracts base name",
			benchmarkName: "BenchmarkSwissMapPresized/Presized-16",
			wantEmpty:     false,
			contains:      "presizing",
		},

		// Known stdlib benchmarks with descriptions
		{
			name:          "JSON encode has description",
			benchmarkName: "BenchmarkJSONEncode",
			wantEmpty:     false,
			contains:      "JSON encoding",
		},
		{
			name:          "IO ReadAll has description",
			benchmarkName: "BenchmarkIOReadAll",
			wantEmpty:     false,
			contains:      "io.ReadAll",
		},
		{
			name:          "IO ReadAll with sub-benchmark has description",
			benchmarkName: "BenchmarkIOReadAll/Size1KB-16",
			wantEmpty:     false,
			contains:      "io.ReadAll",
		},
		{
			name:          "AES CTR has description",
			benchmarkName: "BenchmarkAESCTR",
			wantEmpty:     false,
			contains:      "AES-CTR",
		},
		{
			name:          "AES GCM has description",
			benchmarkName: "BenchmarkAESGCM",
			wantEmpty:     false,
			contains:      "AES-GCM",
		},
		{
			name:          "SHA has description",
			benchmarkName: "BenchmarkSHA",
			wantEmpty:     false,
			contains:      "SHA",
		},
		{
			name:          "RSA key gen has description",
			benchmarkName: "BenchmarkRSAKeyGen",
			wantEmpty:     false,
			contains:      "RSA",
		},
		{
			name:          "Regexp has description",
			benchmarkName: "BenchmarkRegexp",
			wantEmpty:     false,
			contains:      "Regular expression",
		},
		{
			name:          "Buffered IO has description",
			benchmarkName: "BenchmarkBufferedIO",
			wantEmpty:     false,
			contains:      "Buffered",
		},
		{
			name:          "CRC32 has description",
			benchmarkName: "BenchmarkCRC32",
			wantEmpty:     false,
			contains:      "CRC32",
		},
		{
			name:          "FNV hash has description",
			benchmarkName: "BenchmarkFNVHash",
			wantEmpty:     false,
			contains:      "FNV",
		},
		{
			name:          "Binary encode has description",
			benchmarkName: "BenchmarkBinaryEncode",
			wantEmpty:     false,
			contains:      "Binary",
		},
		// Legacy names
		{
			name:          "ReadAll legacy has description",
			benchmarkName: "BenchmarkReadAll",
			wantEmpty:     false,
			contains:      "io.ReadAll",
		},
		{
			name:          "AES CTR encrypt legacy has description",
			benchmarkName: "BenchmarkAESCTREncrypt",
			wantEmpty:     false,
			contains:      "AES",
		},

		// Known networking benchmarks with descriptions
		{
			name:          "TCP connect has description",
			benchmarkName: "BenchmarkTCPConnect",
			wantEmpty:     false,
			contains:      "TCP",
		},
		{
			name:          "TCP throughput has description",
			benchmarkName: "BenchmarkTCPThroughput",
			wantEmpty:     false,
			contains:      "throughput",
		},
		{
			name:          "TLS handshake has description",
			benchmarkName: "BenchmarkTLSHandshake",
			wantEmpty:     false,
			contains:      "TLS",
		},
		{
			name:          "HTTP request has description",
			benchmarkName: "BenchmarkHTTPRequest",
			wantEmpty:     false,
			contains:      "HTTP",
		},

		// Unknown benchmarks return empty string
		{
			name:          "Unknown benchmark has no description",
			benchmarkName: "BenchmarkUnknown",
			wantEmpty:     true,
		},
		{
			name:          "Custom benchmark has no description",
			benchmarkName: "BenchmarkCustomOperation",
			wantEmpty:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getBenchmarkDescription(tt.benchmarkName)
			if tt.wantEmpty {
				if got != "" {
					t.Errorf("getBenchmarkDescription(%q) = %q, want empty string", tt.benchmarkName, got)
				}
			} else {
				if got == "" {
					t.Errorf("getBenchmarkDescription(%q) returned empty, want non-empty", tt.benchmarkName)
				}
				if tt.contains != "" && !containsIgnoreCase(got, tt.contains) {
					t.Errorf("getBenchmarkDescription(%q) = %q, want to contain %q", tt.benchmarkName, got, tt.contains)
				}
			}
		})
	}
}

func containsIgnoreCase(s, substr string) bool {
	s = toLower(s)
	substr = toLower(substr)
	return contains(s, substr)
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c = c + ('a' - 'A')
		}
		result[i] = c
	}
	return string(result)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || indexOfSubstring(s, substr) >= 0)
}

func indexOfSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func TestPlatformDisplayName(t *testing.T) {
	tests := []struct {
		platform string
		want     string
	}{
		{"darwin-arm64", "macOS arm64"},
		{"darwin-amd64", "macOS amd64"},
		{"linux-amd64", "Linux amd64"},
		{"linux-arm64", "Linux arm64"},
		{"windows-amd64", "Windows amd64"},
		{"freebsd-amd64", "FreeBSD amd64"},
		{"openbsd-amd64", "openbsd amd64"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.platform, func(t *testing.T) {
			got := platformDisplayName(tt.platform)
			if got != tt.want {
				t.Errorf("platformDisplayName(%q) = %q, want %q", tt.platform, got, tt.want)
			}
		})
	}
}

func TestUpdatePlatformsJSON(t *testing.T) {
	tmpDir := t.TempDir()

	// First run: create platforms.json with darwin-arm64
	if err := updatePlatformsJSON(tmpDir, "darwin-arm64"); err != nil {
		t.Fatalf("updatePlatformsJSON (first) failed: %v", err)
	}

	data, err := os.ReadFile(tmpDir + "/platforms.json")
	if err != nil {
		t.Fatalf("failed to read platforms.json: %v", err)
	}

	var pd PlatformsData
	if err := json.Unmarshal(data, &pd); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(pd.Platforms) != 1 {
		t.Fatalf("expected 1 platform, got %d", len(pd.Platforms))
	}
	if pd.Platforms[0].Name != "darwin-arm64" {
		t.Errorf("expected darwin-arm64, got %s", pd.Platforms[0].Name)
	}
	if pd.Platforms[0].Display != "macOS arm64" {
		t.Errorf("expected 'macOS arm64', got %s", pd.Platforms[0].Display)
	}
	if pd.Platforms[0].Index != "darwin-arm64/index.json" {
		t.Errorf("expected 'darwin-arm64/index.json', got %s", pd.Platforms[0].Index)
	}

	// Second run: add linux-amd64
	if err := updatePlatformsJSON(tmpDir, "linux-amd64"); err != nil {
		t.Fatalf("updatePlatformsJSON (second) failed: %v", err)
	}

	data, err = os.ReadFile(tmpDir + "/platforms.json")
	if err != nil {
		t.Fatalf("failed to read platforms.json: %v", err)
	}

	if err := json.Unmarshal(data, &pd); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(pd.Platforms) != 2 {
		t.Fatalf("expected 2 platforms, got %d", len(pd.Platforms))
	}

	// Should be sorted alphabetically
	if pd.Platforms[0].Name != "darwin-arm64" {
		t.Errorf("expected first platform darwin-arm64, got %s", pd.Platforms[0].Name)
	}
	if pd.Platforms[1].Name != "linux-amd64" {
		t.Errorf("expected second platform linux-amd64, got %s", pd.Platforms[1].Name)
	}

	// Third run: update existing platform (should not duplicate)
	if err := updatePlatformsJSON(tmpDir, "darwin-arm64"); err != nil {
		t.Fatalf("updatePlatformsJSON (third) failed: %v", err)
	}

	data, err = os.ReadFile(tmpDir + "/platforms.json")
	if err != nil {
		t.Fatalf("failed to read platforms.json: %v", err)
	}

	if err := json.Unmarshal(data, &pd); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(pd.Platforms) != 2 {
		t.Fatalf("expected 2 platforms after update, got %d", len(pd.Platforms))
	}
}

func TestCompareVersionStrings(t *testing.T) {
	tests := []struct {
		a, b string
		want int // -1, 0, or 1
	}{
		// Basic ordering
		{"1.6", "1.24", -1},
		{"1.24", "1.6", 1},
		{"1.24", "1.24", 0},
		// Patch-level ordering
		{"1.24", "1.24.0", 0},
		{"1.24.1", "1.24.2", -1},
		{"1.24.2", "1.24.1", 1},
		{"1.24.0", "1.24.1", -1},
		// Major version ordering
		{"1.25", "2.0", -1},
		{"2.0", "1.25", 1},
		// Three-part vs two-part
		{"1.24.1", "1.25", -1},
		{"1.25", "1.24.1", 1},
		// Empty strings treated as zero
		{"", "1.0", -1},
		{"1.0", "", 1},
	}

	for _, tt := range tests {
		got := compareVersionStrings(tt.a, tt.b)
		// Normalise to -1/0/1 for comparison
		if got < 0 {
			got = -1
		} else if got > 0 {
			got = 1
		}
		if got != tt.want {
			t.Errorf("compareVersionStrings(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestVersionFromJSONFilename(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"go1.24.json", "1.24"},
		{"go1.24.0.json", "1.24.0"},
		{"go1.26.json", "1.26"},
		{"go1.23.json", "1.23"},
	}

	for _, tt := range tests {
		got := versionFromJSONFilename(tt.filename)
		if got != tt.want {
			t.Errorf("versionFromJSONFilename(%q) = %q, want %q", tt.filename, got, tt.want)
		}
	}
}

func TestRebuildIndex(t *testing.T) {
	tmpDir := t.TempDir()
	platformDir := tmpDir + "/linux-amd64"
	if err := os.MkdirAll(platformDir, 0755); err != nil {
		t.Fatalf("failed to create platform dir: %v", err)
	}

	// Helper to write a synthetic version JSON.
	writeVersion := func(filename, version string, benchmarks map[string]Benchmark) {
		t.Helper()
		vd := VersionData{
			Version: version,
			Metadata: VersionMetadata{
				CollectedAt: "2025-01-01T00:00:00Z",
				System:      SystemInfo{OS: "linux", Arch: "amd64"},
			},
			Benchmarks: benchmarks,
		}
		data, err := json.Marshal(vd)
		if err != nil {
			t.Fatalf("failed to marshal version data: %v", err)
		}
		if err := os.WriteFile(platformDir+"/"+filename, data, 0644); err != nil {
			t.Fatalf("failed to write %s: %v", filename, err)
		}
	}

	writeVersion("go1.23.json", "1.23", map[string]Benchmark{
		"BenchmarkFoo": {Name: "BenchmarkFoo", NsPerOp: 100, NsPerOpVariance: 0.02},
		"BenchmarkBar": {Name: "BenchmarkBar", NsPerOp: 200, NsPerOpVariance: 0.12},
	})
	writeVersion("go1.24.json", "1.24", map[string]Benchmark{
		"BenchmarkFoo": {Name: "BenchmarkFoo", NsPerOp: 95, NsPerOpVariance: 0.03},
		"BenchmarkBar": {Name: "BenchmarkBar", NsPerOp: 190, NsPerOpVariance: 0.08},
	})

	// Stale duplicate for 1.24 — should be skipped (older mtime via write order).
	writeVersion("go1.24.0.json", "1.24", map[string]Benchmark{
		"BenchmarkFoo": {Name: "BenchmarkFoo", NsPerOp: 90, NsPerOpVariance: 0.01},
	})

	if err := rebuildIndex(platformDir, tmpDir, "linux-amd64"); err != nil {
		t.Fatalf("rebuildIndex failed: %v", err)
	}

	data, err := os.ReadFile(platformDir + "/index.json")
	if err != nil {
		t.Fatalf("failed to read index.json: %v", err)
	}

	var idx IndexData
	if err := json.Unmarshal(data, &idx); err != nil {
		t.Fatalf("failed to unmarshal index.json: %v", err)
	}

	// Expect exactly 2 unique versions.
	if len(idx.Versions) != 2 {
		t.Fatalf("expected 2 versions, got %d: %v", len(idx.Versions), idx.Versions)
	}

	// Versions should be sorted ascending.
	if idx.Versions[0].Version != "1.23" || idx.Versions[1].Version != "1.24" {
		t.Errorf("unexpected version order: %v", idx.Versions)
	}

	// benchmarkMaxCV for BenchmarkBar is max(0.12, 0.08) = 0.12 → noisy.
	// benchmarkMaxCV for BenchmarkFoo is max(0.02, 0.03) = 0.03 → reliable.
	reliabilityFor := func(name string) string {
		for _, b := range idx.Benchmarks {
			if b.Name == name {
				return b.Reliability
			}
		}
		return ""
	}

	if r := reliabilityFor("BenchmarkBar"); r != "noisy" {
		t.Errorf("BenchmarkBar reliability = %q, want %q", r, "noisy")
	}
	if r := reliabilityFor("BenchmarkFoo"); r != "reliable" {
		t.Errorf("BenchmarkFoo reliability = %q, want %q", r, "reliable")
	}

	// platforms.json should have been created.
	if _, err := os.Stat(tmpDir + "/platforms.json"); err != nil {
		t.Errorf("platforms.json not created: %v", err)
	}
}

// TestAllBenchmarksWithDescriptionsHaveCategories ensures that every benchmark
// with a description also has a category assigned
func TestAllBenchmarksWithDescriptionsHaveCategories(t *testing.T) {
	// Get all benchmark names that have descriptions
	testBenchmarks := []string{
		// Runtime/GC benchmarks
		"BenchmarkSmallAllocation",
		"BenchmarkMapCreation",
		"BenchmarkSwissMapCreation",
		"BenchmarkSwissMapLarge",
		"BenchmarkSwissMapPresized",
		"BenchmarkSwissMapIteration",
		"BenchmarkSmallAllocSpecialized",
		"BenchmarkSyncMap",
		"BenchmarkGCThroughput",
		"BenchmarkGCLatency",
		"BenchmarkGCLatencyP99",
		"BenchmarkSmallObjectScanning",
		"BenchmarkMediumObjectScanning",
		"BenchmarkLargeObjectScanning",
		"BenchmarkAtomicIncrement",
		"BenchmarkMutexContention",
		"BenchmarkChannelThroughput",
		"BenchmarkGCMixedWorkload",
		"BenchmarkGCSmallObjects",
		"BenchmarkGoroutineCreate",
		"BenchmarkStackGrowth",

		// Standard library benchmarks (actual names)
		"BenchmarkJSONEncode",
		"BenchmarkJSONDecode",
		"BenchmarkJSONDecodeStream",
		"BenchmarkIOReadAll",
		"BenchmarkAESCTR",
		"BenchmarkAESGCM",
		"BenchmarkSHA",
		"BenchmarkRSAKeyGen",
		"BenchmarkRegexp",
		"BenchmarkBufferedIO",
		"BenchmarkCRC32",
		"BenchmarkFNVHash",
		"BenchmarkBinaryEncode",
		"BenchmarkStringsJoin",
		// Legacy names for backwards compatibility
		"BenchmarkReadAll",
		"BenchmarkReadAllLarge",
		"BenchmarkAESCTREncrypt",
		"BenchmarkSHA1Hash",
		"BenchmarkSHA3Hash",
		"BenchmarkRSAKeyGeneration",
		"BenchmarkRegexpMatch",
		"BenchmarkRegexpCompile",

		// Networking benchmarks
		"BenchmarkTCPConnect",
		"BenchmarkTCPKeepAlive",
		"BenchmarkTCPThroughput",
		"BenchmarkTLSHandshake",
		"BenchmarkTLSResume",
		"BenchmarkTLSThroughput",
		"BenchmarkHTTP2",
		"BenchmarkHTTPRequest",
		"BenchmarkConnectionPool",

		// Legacy runtime benchmarks
		"BenchmarkLargeAllocation",
		"BenchmarkMapAllocation",
		"BenchmarkSliceAppend",
		"BenchmarkGCPressure",
	}

	for _, benchName := range testBenchmarks {
		t.Run(benchName, func(t *testing.T) {
			desc := getBenchmarkDescription(benchName)
			category := getBenchmarkCategory(benchName)

			if desc != "" && category == "uncategorized" {
				t.Errorf("Benchmark %q has description but no category assigned", benchName)
			}
		})
	}
}
