package stdlib

import (
	"hash/crc32"
	"hash/fnv"
	"testing"
)

// Pre-generated deterministic hash test data
var (
	hashData64KB []byte
	hashData1KB  []byte
)

func init() {
	// Generate deterministic hash data
	hashData1KB = make([]byte, 1024)
	for i := range hashData1KB {
		hashData1KB[i] = byte(i % 256)
	}

	hashData64KB = make([]byte, 64*1024)
	for i := range hashData64KB {
		hashData64KB[i] = byte(i % 256)
	}
}

// BenchmarkCRC32 measures CRC32 checksum calculation performance.
// CRC32 uses platform-specific optimizations - Castagnoli uses SSE4.2 on x86.
func BenchmarkCRC32(b *testing.B) {
	b.Run("IEEE", func(b *testing.B) {
		b.ReportAllocs()
		table := crc32.MakeTable(crc32.IEEE)
		b.SetBytes(int64(len(hashData64KB)))

		for b.Loop() {
			sum := crc32.Checksum(hashData64KB, table)
			_ = sum
		}
	})

	b.Run("Castagnoli", func(b *testing.B) {
		b.ReportAllocs()
		table := crc32.MakeTable(crc32.Castagnoli)
		b.SetBytes(int64(len(hashData64KB)))

		for b.Loop() {
			sum := crc32.Checksum(hashData64KB, table)
			_ = sum
		}
	})
}

// BenchmarkFNVHash measures FNV hash function performance.
// FNV-1a hash is commonly used for hash tables - baseline for internal hash performance.
func BenchmarkFNVHash(b *testing.B) {
	b.Run("FNV1a_64", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(hashData1KB)))

		for b.Loop() {
			h := fnv.New64a()
			h.Write(hashData1KB)
			sum := h.Sum64()
			_ = sum
		}
	})

	b.Run("FNV1a_128", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(hashData1KB)))

		for b.Loop() {
			h := fnv.New128a()
			h.Write(hashData1KB)
			sum := h.Sum(nil)
			_ = sum
		}
	})
}
