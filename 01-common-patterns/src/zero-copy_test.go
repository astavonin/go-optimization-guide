
package perf

import (
    "io"
    "os"
    "testing"

    "golang.org/x/exp/mmap"
)

// bench-start
var sink []byte

func BenchmarkCopy(b *testing.B) {
    data := make([]byte, 64*1024)
    for i := 0; i < b.N; i++ {
        buf := make([]byte, len(data))
        copy(buf, data)
        sink = buf
    }
}

func BenchmarkSlice(b *testing.B) {
    data := make([]byte, 64*1024)
    for i := 0; i < b.N; i++ {
        s := data[:]
        sink = s
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
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := f.ReadAt(buf, 0)
        if err != nil && err != io.EOF {
            b.Fatal(err)
        }
        sink = buf
    }
}

func BenchmarkReadWithMmap(b *testing.B) {
    r, err := mmap.Open("testdata/largefile.bin")
    if err != nil {
        b.Fatalf("failed to mmap file: %v", err)
    }
    defer r.Close()

    buf := make([]byte, r.Len())
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := r.ReadAt(buf, 0)
        if err != nil && err != io.EOF {
            b.Fatal(err)
        }
        sink = buf
    }
}
// bench-io-end