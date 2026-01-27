package main

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"golang.org/x/sys/unix"
)

const bufSize = 1 << 20 // 1MB

// memory-intensive loop that touches buffer
func touchBuffer(buf []byte) {
	for i := 0; i < len(buf); i += 64 {
		buf[i]++
	}
}

func BenchmarkBufferAccess_Pinned(b *testing.B) {
	numCPU := runtime.GOMAXPROCS(0)
	var wg sync.WaitGroup
	var counter int64

	for i := 0; i < numCPU; i++ {
		wg.Add(1)
		go func() {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			defer wg.Done()

			buf := make([]byte, bufSize)

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

func BenchmarkBufferAccess_PinnedWithAffinity(b *testing.B) {
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

			buf := make([]byte, bufSize)

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

func BenchmarkBufferAccess_GoParallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		buf := make([]byte, bufSize)
		for pb.Next() {
			touchBuffer(buf)
		}
	})
}

// Linux-specific
func setAffinity(cpu int) error {
	var mask unix.CPUSet
	mask.Set(cpu)
	return unix.SchedSetaffinity(unix.Gettid(), &mask)
}