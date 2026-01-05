package core

import "testing"

// BenchmarkSmallAllocation tracks small object allocation performance.
// Go 1.26 shows ~30% improvement for small allocations.
func BenchmarkSmallAllocation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = make([]byte, 64)
	}
}

// BenchmarkLargeAllocation tests allocation at scale.
func BenchmarkLargeAllocation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = make([]byte, 1<<20) // 1MB
	}
}

// BenchmarkMapAllocation measures map allocation patterns.
// Maps are sensitive to GC changes.
func BenchmarkMapAllocation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		m := make(map[int]int, 100)
		for j := range 100 {
			m[j] = j
		}
	}
}

// BenchmarkSliceAppend tracks slice growth patterns.
// Go 1.25 improved slice backing store allocation.
func BenchmarkSliceAppend(b *testing.B) {
	for i := 0; i < b.N; i++ {
		s := make([]int, 0)
		for j := range 1000 {
			s = append(s, j)
		}
		_ = s // Prevent optimization
	}
}

// BenchmarkGCPressure measures GC behavior under allocation pressure.
// Sensitive to Green Tea GC improvements in Go 1.25/1.26.
func BenchmarkGCPressure(b *testing.B) {
	var sink [][]byte
	for i := 0; i < b.N; i++ {
		sink = append(sink, make([]byte, 1024))
		if len(sink) > 100 {
			sink = sink[:0]
		}
	}
}
