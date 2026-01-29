package main

import "testing"

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
