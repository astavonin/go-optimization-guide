package perf

import (
    "testing"
)

func BenchmarkAppendNoPrealloc(b *testing.B) {
    for i := 0; i < b.N; i++ {
        var s []int
        for j := 0; j < 10000; j++ {
            s = append(s, j)
        }
    }
}

func BenchmarkAppendWithPrealloc(b *testing.B) {
    for i := 0; i < b.N; i++ {
        s := make([]int, 0, 10000)
        for j := 0; j < 10000; j++ {
            s = append(s, j)
        }
    }
}