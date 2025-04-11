
package perf

import (
    "io"
    "os"
    "testing"

    "golang.org/x/exp/mmap"
)

// bench-start
func BenchmarkCopy(b *testing.B) {
    data := make([]byte, 64*1024)
    for b.Loop() {
        buf := make([]byte, len(data))
        copy(buf, data)
    }
}

func BenchmarkSlice(b *testing.B) {
    data := make([]byte, 64*1024)
    for b.Loop() {
        _ = data[:]
    }
}
// bench-end

// bench-io-start
func BenchmarkReadWithCopy(b *testing.B) {
    f, err := os.Open("testdata/largefile.bin")
    if err != nil {
        b.Fatalf("failed to open file: %v", err)
    }
    defer f.Close()

    buf := make([]byte, 4*1024*1024) // 4MB buffer
    for b.Loop() {
        _, err := f.ReadAt(buf, 0)
        if err != nil && err != io.EOF {
            b.Fatal(err)
        }
    }
}

func BenchmarkReadWithMmap(b *testing.B) {
    r, err := mmap.Open("testdata/largefile.bin")
    if err != nil {
        b.Fatalf("failed to mmap file: %v", err)
    }
    defer r.Close()

    buf := make([]byte, r.Len())
    for b.Loop() {
        _, err := r.ReadAt(buf, 0)
        if err != nil && err != io.EOF {
            b.Fatal(err)
        }
    }
}
// bench-io-end
