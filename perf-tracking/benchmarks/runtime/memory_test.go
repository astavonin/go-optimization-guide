package runtime

import (
	"testing"
)

// BenchmarkStackGrowth measures stack allocation and growth patterns.
// Go 1.23+ includes stack frame slot overlap optimization.
func BenchmarkStackGrowth(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = recursive(100)
	}
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
	sizes := []int{32, 64, 128, 256, 512}

	for _, size := range sizes {
		b.Run(allocSizeToString(size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = make([]byte, size)
			}
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
	for i := 0; i < b.N; i++ {
		done := make(chan struct{})
		go func() {
			close(done)
		}()
		<-done
	}
}
