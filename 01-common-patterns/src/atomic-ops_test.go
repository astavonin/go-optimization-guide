package perf

import (
    "testing"
	"sync/atomic"
	"sync"
)

// bench-start
func BenchmarkAtomicIncrement(b *testing.B) {
	var counter int64
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			atomic.AddInt64(&counter, 1)
		}
	})
}

func BenchmarkMutexIncrement(b *testing.B) {
	var (
		counter int64
		mu      sync.Mutex
	)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mu.Lock()
			counter++
			mu.Unlock()
		}
	})
}
// bench-end