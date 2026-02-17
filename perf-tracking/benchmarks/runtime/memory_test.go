package runtime

import (
	"runtime"
	"testing"
	"unsafe"
)

var (
	sinkBytes []byte
	sinkInt   int
)

// BenchmarkStackGrowth measures stack allocation and growth patterns.
// Go 1.23+ includes stack frame slot overlap optimization.
// Note: Uses a fresh goroutine per iteration to force stack growth.
func BenchmarkStackGrowth(b *testing.B) {
	b.ReportAllocs()
	resultCh := make(chan int, 1)
	for b.Loop() {
		go func() {
			resultCh <- recursive(100)
		}()
		sinkInt = <-resultCh
	}
	_ = unsafe.Pointer(&sinkInt)
}

// recursive forces stack expansion for testing stack growth.
func recursive(n int) int {
	if n == 0 {
		return 0
	}
	arr := [128]int{} // Stack-allocated array
	return arr[0] + recursive(n-1)
}

// BenchmarkSmallAllocSpecialized measures sub-512 byte allocation performance.
// Go 1.26 shows up to 30% improvement for allocations under 512 bytes.
func BenchmarkSmallAllocSpecialized(b *testing.B) {
	runtime.GC()
	b.ResetTimer()
	sizes := []int{32, 64, 128, 256, 512}

	for _, size := range sizes {
		b.Run(allocSizeToString(size), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(size))
			for b.Loop() {
				sinkBytes = make([]byte, size)
			}
			_ = unsafe.Pointer(&sinkBytes)
		})
	}
}

// allocSizeToString converts size to string for sub-benchmark names.
func allocSizeToString(size int) string {
	switch size {
	case 32:
		return "Size32"
	case 64:
		return "Size64"
	case 128:
		return "Size128"
	case 256:
		return "Size256"
	case 512:
		return "Size512"
	default:
		return "Unknown"
	}
}

// BenchmarkGoroutineCreate measures goroutine creation overhead.
// Baseline for scheduler performance across versions.
func BenchmarkGoroutineCreate(b *testing.B) {
	for b.Loop() {
		done := make(chan struct{})
		go func() {
			close(done)
		}()
		<-done
	}
}
