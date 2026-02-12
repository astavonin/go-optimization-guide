package main

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"golang.org/x/sys/unix"
)

const (
	buf2MB = 2 << 20 // 2MB
	buf4MB = 4 << 20 // 4MB
)

// touchBuffer simulates thread-local hot data access
func touchBuffer(buf []byte) {
	for i := 0; i < len(buf); i += 64 {
		buf[i]++
	}
}

func BenchmarkBufferAccess_GoParallel_2MB(b *testing.B)   { runGoParallelBuffer(b, buf2MB) }
func BenchmarkBufferAccess_GoParallel_4MB(b *testing.B)   { runGoParallelBuffer(b, buf4MB) }
func BenchmarkBufferAccess_Pinned_2MB(b *testing.B)       { runPinnedBuffer(b, buf2MB) }
func BenchmarkBufferAccess_Pinned_4MB(b *testing.B)       { runPinnedBuffer(b, buf4MB) }
func BenchmarkBufferAccess_PinnedWithAffinity_2MB(b *testing.B) { runPinnedAffinityBuffer(b, buf2MB) }
func BenchmarkBufferAccess_PinnedWithAffinity_4MB(b *testing.B) { runPinnedAffinityBuffer(b, buf4MB) }

// standard Go scheduler parallelism
func runGoParallelBuffer(b *testing.B, size int) {
	b.RunParallel(func(pb *testing.PB) {
		buf := make([]byte, size)
		for pb.Next() {
			touchBuffer(buf)
		}
	})
}

// pinned thread benchmark
func runPinnedBuffer(b *testing.B, size int) {
	numCPU := runtime.GOMAXPROCS(0)
	var wg sync.WaitGroup
	var counter int64

	for i := 0; i < numCPU; i++ {
		wg.Add(1)
		go func() {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			defer wg.Done()

			buf := make([]byte, size)

			for {
				if atomic.AddInt64(&counter, 1) > int64(b.N) {
					break
				}
				touchBuffer(buf)
			}
		}()
	}
	wg.Wait()
}

// pinned + affinity
func runPinnedAffinityBuffer(b *testing.B, size int) {
	numCPU := runtime.GOMAXPROCS(0)
	var wg sync.WaitGroup
	var counter int64

	for i := 0; i < numCPU; i++ {
		wg.Add(1)
		go func(cpu int) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			defer wg.Done()

			if err := setAffinity(cpu); err != nil {
				panic(err)
			}

			buf := make([]byte, size)

			for {
				if atomic.AddInt64(&counter, 1) > int64(b.N) {
					break
				}
				touchBuffer(buf)
			}
		}(i)
	}
	wg.Wait()
}

// linux-only
func setAffinity(cpu int) error {
	var mask unix.CPUSet
	mask.Set(cpu)
	return unix.SchedSetaffinity(unix.Gettid(), &mask)
}