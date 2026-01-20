package runtime

import (
	"math/rand"
	"sync"
	"testing"
)

// BenchmarkSyncMap measures sync.Map concurrent operations.
// Go 1.24 reduces contention for disjoint key access patterns.
func BenchmarkSyncMap(b *testing.B) {
	b.Run("SingleThreaded", func(b *testing.B) {
		var m sync.Map
		for i := 0; i < b.N; i++ {
			m.Store(i%1000, i)
			_, _ = m.Load(i % 1000)
		}
	})

	b.Run("Parallel", func(b *testing.B) {
		var m sync.Map
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				m.Store(i%1000, i)
				_, _ = m.Load(i % 1000)
				i++
			}
		})
	})
}

// BenchmarkSwissMapLarge measures large map access with Swiss Tables.
// Go 1.24+ shows ~30% faster access for large maps.
func BenchmarkSwissMapLarge(b *testing.B) {
	// Pre-populate map with 10,000 entries
	m := make(map[int]int)
	for i := range 10000 {
		m[i] = i
	}

	// Shuffled index list for deterministic access
	indices := make([]int, 10000)
	for i := range indices {
		indices[i] = i
	}
	// Shuffle with fixed seed for reproducibility
	rng := rand.New(rand.NewSource(42))
	rng.Shuffle(len(indices), func(i, j int) {
		indices[i], indices[j] = indices[j], indices[i]
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m[indices[i%10000]]
	}
}

// BenchmarkSwissMapPresized measures pre-allocated map performance.
// Swiss Tables benefit more from pre-sizing in Go 1.24+.
func BenchmarkSwissMapPresized(b *testing.B) {
	b.Run("Presized", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			m := make(map[int]int, 1000) // Pre-sized
			for j := range 1000 {
				m[j] = j
			}
			_ = m // Prevent DCE
		}
	})

	b.Run("GrowAsNeeded", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			m := make(map[int]int) // No pre-sizing
			for j := range 1000 {
				m[j] = j
			}
			_ = m // Prevent DCE
		}
	})
}

// BenchmarkSwissMapIteration measures map range iteration speed.
// Go 1.24+ shows 10-60% faster iteration depending on map size.
func BenchmarkSwissMapIteration(b *testing.B) {
	sizes := []int{100, 1000, 10000}

	for _, size := range sizes {
		b.Run(sizeToString(size), func(b *testing.B) {
			m := make(map[int]int)
			for i := range size {
				m[i] = i
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sum := 0
				for _, v := range m {
					sum += v
				}
				_ = sum // Prevent DCE
			}
		})
	}
}

// sizeToString converts size to string for sub-benchmark names.
func sizeToString(size int) string {
	switch size {
	case 100:
		return "Size100"
	case 1000:
		return "Size1000"
	case 10000:
		return "Size10000"
	default:
		return "Unknown"
	}
}

// BenchmarkMutexContention measures mutex performance under contention.
// Baseline for scheduler behavior across versions.
func BenchmarkMutexContention(b *testing.B) {
	var mu sync.Mutex
	var counter int

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mu.Lock()
			counter++
			mu.Unlock()
		}
	})

	_ = counter // Prevent DCE
}
