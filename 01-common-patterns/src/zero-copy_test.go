package perf

import (
	"io"
	"os"
	"testing"

	"crypto/sha256"

	"github.com/cespare/xxhash/v2"
	"golang.org/x/exp/mmap"
	"golang.org/x/sys/unix"
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

func BenchmarkReadAtCopySHA(b *testing.B) {
	f, err := os.Open("testdata/largefile.bin")
	if err != nil {
		b.Fatalf("open: %v", err)
	}
	defer f.Close()

// bench-sha-start
	buf := make([]byte, 4*1024*1024)

	b.ResetTimer()
	for b.Loop() {
		_, err := f.ReadAt(buf, 0)
		if err != nil && err != io.EOF {
			b.Fatal(err)
		}
		_ = sha256.Sum256(buf) // consume so compiler can't DCE everything
	}
// bench-sha-end
}

func BenchmarkMmapNoCopySHA(b *testing.B) {
	f, err := os.Open("testdata/largefile.bin")
	if err != nil {
		b.Fatalf("open: %v", err)
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		b.Fatalf("stat: %v", err)
	}
	size := int(st.Size())
	if size == 0 {
		b.Fatal("empty file")
	}

// bench-sha-mmap-start
	data, err := unix.Mmap(int(f.Fd()), 0, size, unix.PROT_READ, unix.MAP_SHARED)
	if err != nil {
		b.Fatalf("mmap: %v", err)
	}
	defer func() {
		if err := unix.Munmap(data); err != nil {
			b.Fatalf("munmap: %v", err)
		}
	}()

	window := data
	if len(window) > 4*1024*1024 {
		window = window[:4*1024*1024] // match the 4MB workload shape
	}

	b.ResetTimer()
	for b.Loop() {
		_ = sha256.Sum256(window) // reads directly from mapped pages, no extra copy
	}
// bench-sha-mmap-end
}

func BenchmarkReadAtCopyXXHash(b *testing.B) {
	f, err := os.Open("testdata/largefile.bin")
	if err != nil {
		b.Fatalf("open: %v", err)
	}
	defer f.Close()

// bench-hash-start
	buf := make([]byte, 4*1024*1024)

	b.ResetTimer()
	for b.Loop() {
		_, err := f.ReadAt(buf, 0)
		if err != nil && err != io.EOF {
			b.Fatal(err)
		}
		h := xxhash.New()
		h.Write(buf)
		_ = h.Sum64() // consume to prevent DCE
	}
// bench-hash-end
}

func BenchmarkMmapNoCopyXXHash(b *testing.B) {
	f, err := os.Open("testdata/largefile.bin")
	if err != nil {
		b.Fatalf("open: %v", err)
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		b.Fatalf("stat: %v", err)
	}
	size := int(st.Size())
	if size == 0 {
		b.Fatal("empty file")
	}

// bench-hash-mmap-start
	data, err := unix.Mmap(int(f.Fd()), 0, size, unix.PROT_READ, unix.MAP_SHARED)
	if err != nil {
		b.Fatalf("mmap: %v", err)
	}
	defer func() {
		if err := unix.Munmap(data); err != nil {
			b.Fatalf("munmap: %v", err)
		}
	}()

	window := data
	if len(window) > 4*1024*1024 {
		window = window[:4*1024*1024] // match the 4MB workload shape
	}

	b.ResetTimer()
	for b.Loop() {
		h := xxhash.New()
		h.Write(window) // reads directly from mapped pages, no extra copy
		_ = h.Sum64()
	}
// bench-hash-mmap-end
}
