package main

import (
	"io"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

func mixedLoop(b *testing.B, pinned bool, r io.Reader) {
	var counter int64
	var wg sync.WaitGroup
	// numCPU := runtime.GOMAXPROCS(0)

	wg.Add(1)
	go func() {
		if pinned {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
		}
		defer wg.Done()

		buf := make([]byte, 1024) // blocking read size
		for atomic.LoadInt64(&counter) < int64(b.N) {
			// CPU‑bound segment
			for i := 0; i < 1000; i++ {
				_ = i * i
			}
			// Blocking I/O
			r.Read(buf)
			atomic.AddInt64(&counter, 1)
		}
	}()
	wg.Wait()
}

func BenchmarkMixed_Unpinned(b *testing.B) {
	r, w, _ := os.Pipe()
	// background writer to unblock reads
	go func() {
		for {
			w.Write([]byte("x"))
		}
	}()
	for i := 0; i < b.N; i++ {
		mixedLoop(b, false, r)
	}
}

func BenchmarkMixed_Pinned(b *testing.B) {
	r, w, _ := os.Pipe()
	go func() {
		for {
			w.Write([]byte("x"))
		}
	}()
	for i := 0; i < b.N; i++ {
		mixedLoop(b, true, r)
	}
}