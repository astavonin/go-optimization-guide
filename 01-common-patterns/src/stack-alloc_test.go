package main

import "testing"

type Data struct {
    A, B, C int
}

// heap-alloc-start
func StackAlloc() Data {
    return Data{1, 2, 3} // stays on stack
}

func HeapAlloc() *Data {
    return &Data{1, 2, 3} // escapes to heap
}

func BenchmarkStackAlloc(b *testing.B) {
    for i := 0; i < b.N; i++ {
        _ = StackAlloc()
    }
}

func BenchmarkHeapAlloc(b *testing.B) {
    for i := 0; i < b.N; i++ {
        _ = HeapAlloc()
    }
}
// heap-alloc-end

// escape-start
var sink *Data

func HeapAllocEscape() {
    d := &Data{1, 2, 3}
    sink = d // d escapes to heap
}

func BenchmarkHeapAllocEscape(b *testing.B) {
    for i := 0; i < b.N; i++ {
        HeapAllocEscape()
    }
}
// escape-end
