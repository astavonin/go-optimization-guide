package perf

import (
	"bytes"
	"sync"
	"testing"
)

// requestPayload simulates a fixed-size request body written per iteration.
var requestPayload = bytes.Repeat([]byte("x"), 4096)

// BenchmarkWithoutPooling allocates a fresh bytes.Buffer on every call.
// The buffer's internal backing array is heap-allocated on each Write,
// which is the allocation pattern this benchmark measures.
func BenchmarkWithoutPooling(b *testing.B) {
	for b.Loop() {
		buf := &bytes.Buffer{}
		buf.Write(requestPayload)
		_ = buf.Bytes()
	}
}

// bufPool reuses bytes.Buffer instances across calls. After the first
// iteration the buffer's internal slice is already sized, so subsequent
// iterations avoid heap allocation entirely.
var bufPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// BenchmarkWithPooling retrieves a buffer from the pool, writes into it,
// then returns it. The Reset call repositions the read offset without
// freeing the underlying slice, so no allocation occurs after warm-up.
func BenchmarkWithPooling(b *testing.B) {
	for b.Loop() {
		buf := bufPool.Get().(*bytes.Buffer)
		buf.Reset()
		buf.Write(requestPayload)
		_ = buf.Bytes()
		bufPool.Put(buf)
	}
}
