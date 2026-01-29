package runtime

import (
	"runtime"
	"testing"
)

var sinkData []*Data

// Data represents a test allocation with payload.
type Data struct {
	payload []byte
}

// SmallData represents small object for GC scanning tests.
type SmallData struct {
	value int64
}

// BenchmarkGCThroughput measures allocation throughput under GC pressure.
// Green Tea GC shows 10-40% improvement in Go 1.25/1.26.
func BenchmarkGCThroughput(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(128 * 1000)
	var sink []*Data // Live heap across iterations

	for i := 0; i < b.N; i++ {
		objects := make([]*Data, 1000)
		for j := range 1000 {
			objects[j] = &Data{payload: make([]byte, 128)}
		}
		// Retain some objects to maintain live heap
		sink = append(sink, objects[0:100]...)
		if len(sink) > 10000 {
			sink = sink[1000:] // Keep heap live but bounded
		}
	}

	sinkData = sink // Prevent DCE
}

// BenchmarkGCLatency measures garbage collection pause times.
// Green Tea GC reduces pause times in Go 1.25/1.26.
func BenchmarkGCLatency(b *testing.B) {
	b.ReportAllocs()
	var ms runtime.MemStats
	var sink [][]byte // Retain live heap

	// Warmup and setup
	b.StopTimer()
	for i := 0; i < 100; i++ {
		sink = append(sink, make([]byte, 1024))
	}
	runtime.ReadMemStats(&ms)
	basePauseNs := ms.PauseTotalNs
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		// Allocate burst
		burst := make([][]byte, 1000)
		for j := range 1000 {
			burst[j] = make([]byte, 1024)
		}
		sink = append(sink, burst[0]) // Retain some
		if len(sink) > 1000 {
			sink = sink[100:] // Keep heap live but bounded
		}

		// Force GC and measure pause
		runtime.GC()
	}

	b.StopTimer()
	runtime.ReadMemStats(&ms)
	pauseNs := ms.PauseTotalNs - basePauseNs
	if b.N > 0 {
		b.ReportMetric(float64(pauseNs)/float64(b.N), "pause-ns/gc")
	}
	_ = sink // Prevent DCE
}

// BenchmarkGCSmallObjects measures GC performance scanning small objects.
// Go 1.26 uses vector instructions for improved scanning on modern CPUs.
func BenchmarkGCSmallObjects(b *testing.B) {
	b.ReportAllocs()
	var sink []*SmallData // Retain live heap, use concrete type to avoid interface boxing

	for i := 0; i < b.N; i++ {
		objects := make([]*SmallData, 10000)
		for j := range 10000 {
			objects[j] = &SmallData{value: int64(j)}
		}
		// Retain some objects
		if i%10 == 0 {
			sink = append(sink, objects[0:100]...)
			if len(sink) > 1000 {
				sink = sink[100:]
			}
		}
	}

	_ = sink // Prevent DCE
}

// BenchmarkGCMixedWorkload measures realistic mixed allocation patterns.
// Tests overall GC behavior with small, medium, and large objects.
func BenchmarkGCMixedWorkload(b *testing.B) {
	b.ReportAllocs()
	var sink [][]byte // Retain live heap

	for i := 0; i < b.N; i++ {
		small := make([]byte, 32)
		medium := make([]byte, 4096)
		large := make([]byte, 1<<20)

		// Retain some allocations including large to create realistic GC pressure
		if i%100 == 0 {
			sink = append(sink, small, medium, large)
			if len(sink) > 300 {
				sink = sink[30:]
			}
		}

		_ = small
		_ = medium
		_ = large
	}

	_ = sink // Prevent DCE
}
