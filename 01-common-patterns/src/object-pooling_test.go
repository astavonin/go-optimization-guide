package perf

import (
	"sync"
	"testing"
)

// Data is a struct with a large fixed-size array to simulate a memory-intensive object.
type Data struct {
	Values [1024]int
}

// BenchmarkWithoutPooling measures the performance of direct heap allocations.
func BenchmarkWithoutPooling(b *testing.B) {
	for b.Loop() {
    data := &Data{}      // Allocating a new object each time
		data.Values[0] = 42 // Simulating some memory activity
	}
}

// dataPool is a sync.Pool that reuses instances of Data to reduce memory allocations.
var dataPool = sync.Pool{
	New: func() any {
		return &Data{}
	},
}

// BenchmarkWithPooling measures the performance of using sync.Pool to reuse objects.
func BenchmarkWithPooling(b *testing.B) {
	for b.Loop() {
		obj := dataPool.Get().(*Data) // Retrieve from pool
		obj.Values[0] = 42            // Simulate memory usage
		dataPool.Put(obj)             // Return object to pool for reuse
	}
}
