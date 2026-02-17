package core

import (
	"runtime"
	"testing"
	"unsafe"
)

var (
	sinkBytes []byte
	sinkMap   map[int]int
	sinkInts  []int
)

// BenchmarkSmallAllocation tracks small object allocation performance.
// Go 1.26 shows ~30% improvement for small allocations.
func BenchmarkSmallAllocation(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(64)
	runtime.GC()
	b.ResetTimer()
	for b.Loop() {
		sinkBytes = make([]byte, 64)
	}
	_ = unsafe.Pointer(&sinkBytes)
}

// BenchmarkLargeAllocation tests allocation at scale.
func BenchmarkLargeAllocation(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(1 << 20)
	runtime.GC()
	b.ResetTimer()
	for b.Loop() {
		sinkBytes = make([]byte, 1<<20) // 1MB
	}
	_ = unsafe.Pointer(&sinkBytes)
}

// BenchmarkMapAllocation measures map allocation patterns.
// Maps are sensitive to GC changes.
func BenchmarkMapAllocation(b *testing.B) {
	b.ReportAllocs()
	runtime.GC()
	b.ResetTimer()
	for b.Loop() {
		m := make(map[int]int, 100)
		for j := range 100 {
			m[j] = j
		}
		sinkMap = m
	}
	_ = unsafe.Pointer(&sinkMap)
}

// BenchmarkSliceAppend tracks slice growth patterns.
// Go 1.25 improved slice backing store allocation.
func BenchmarkSliceAppend(b *testing.B) {
	b.ReportAllocs()
	runtime.GC()
	b.ResetTimer()
	for b.Loop() {
		s := make([]int, 0)
		for j := range 1000 {
			s = append(s, j)
		}
		sinkInts = s
	}
	_ = unsafe.Pointer(&sinkInts)
}

// BenchmarkGCPressure measures GC behavior under allocation pressure.
// Sensitive to Green Tea GC improvements in Go 1.25/1.26.
func BenchmarkGCPressure(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(1024)
	var ms runtime.MemStats
	var sink [][]byte
	b.StopTimer()
	runtime.ReadMemStats(&ms)
	basePauseNs := ms.PauseTotalNs
	b.StartTimer()
	var n int
	for b.Loop() {
		sink = append(sink, make([]byte, 1024))
		if len(sink) > 100 {
			sink = sink[:0]
		}
		n++
	}
	b.StopTimer()
	runtime.ReadMemStats(&ms)
	pauseNs := ms.PauseTotalNs - basePauseNs
	if n > 0 {
		b.ReportMetric(float64(pauseNs)/float64(n), "pause-ns/op")
	}
	_ = unsafe.Pointer(&sink)
}
