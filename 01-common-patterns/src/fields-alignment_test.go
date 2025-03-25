package perf

import (
    "sync"
    "testing"
)

// types-simple-start
type PoorlyAligned struct {
    flag bool
    count int64
    id byte
}

type WellAligned struct {
    count int64
    flag bool
    id byte
}
// types-simple-end

var result int64

// simple-start
func BenchmarkPoorlyAligned(b *testing.B) {
    for i := 0; i < b.N; i++ {
        var items = make([]PoorlyAligned, 10_000_000)
        for j := range items {
            items[j].count = int64(j)
            result += items[j].count
        }
    }
}

func BenchmarkWellAligned(b *testing.B) {
    for i := 0; i < b.N; i++ {
        var items = make([]WellAligned, 10_000_000)
        for j := range items {
            items[j].count = int64(j)
            result += items[j].count
        }
    }
}
// simple-end


// types-shared-start
type SharedCounterBad struct {
    a int64
    b int64
}

type SharedCounterGood struct {
    a int64
    _ [56]byte // Padding to prevent a and b from sharing a cache line
    b int64
}
// types-shared-end

// shared-start

func BenchmarkFalseSharing(b *testing.B) {
    var c SharedCounterBad  // (1)
    var wg sync.WaitGroup

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        wg.Add(2)
        go func() {
            for i := 0; i < 1_000_000; i++ {
                c.a++
            }
            wg.Done()
        }()
        go func() {
            for i := 0; i < 1_000_000; i++ {
                c.b++
            }
            wg.Done()
        }()
        wg.Wait()
    }
}
// shared-end

func BenchmarkNoFalseSharing(b *testing.B) {
    var c SharedCounterGood
    var wg sync.WaitGroup

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        wg.Add(2)
        go func() {
            for i := 0; i < 1_000_000; i++ {
                c.a++
            }
            wg.Done()
        }()
        go func() {
            for i := 0; i < 1_000_000; i++ {
                c.b++
            }
            wg.Done()
        }()
        wg.Wait()
    }
}

