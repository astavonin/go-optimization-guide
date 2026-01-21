package stdlib

import (
	"bufio"
	"bytes"
	"io"
	"testing"
)

// Pre-generated deterministic I/O test data
var (
	ioData1KB   []byte
	ioData64KB  []byte
	ioData1MB   []byte
	ioData4KB   []byte
	ioDataWrite []byte
)

func init() {
	// Generate deterministic I/O buffers
	ioData1KB = make([]byte, 1024)
	for i := range ioData1KB {
		ioData1KB[i] = byte(i % 256)
	}

	ioData64KB = make([]byte, 64*1024)
	for i := range ioData64KB {
		ioData64KB[i] = byte(i % 256)
	}

	ioData1MB = make([]byte, 1024*1024)
	for i := range ioData1MB {
		ioData1MB[i] = byte(i % 256)
	}

	ioData4KB = make([]byte, 4096)
	for i := range ioData4KB {
		ioData4KB[i] = byte(i % 256)
	}

	ioDataWrite = make([]byte, 64*1024)
	for i := range ioDataWrite {
		ioDataWrite[i] = byte(i % 256)
	}
}

// BenchmarkIOReadAll measures io.ReadAll performance improvement.
// Go 1.26 shows ~2x faster performance and ~50% less memory allocation.
func BenchmarkIOReadAll(b *testing.B) {
	sizes := []struct {
		name string
		data []byte
	}{
		{"Size1KB", ioData1KB},
		{"Size64KB", ioData64KB},
		{"Size1MB", ioData1MB},
	}

	for _, tc := range sizes {
		b.Run(tc.name, func(b *testing.B) {
			b.SetBytes(int64(len(tc.data)))
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				reader := bytes.NewReader(tc.data)
				result, err := io.ReadAll(reader)
				if err != nil {
					b.Fatal(err)
				}
				_ = result
			}
		})
	}
}

// BenchmarkBufferedIO measures buffered I/O patterns.
// Baseline for comparison across versions.
func BenchmarkBufferedIO(b *testing.B) {
	b.Run("Reader", func(b *testing.B) {
		chunk := make([]byte, 4096)

		for i := 0; i < b.N; i++ {
			reader := bufio.NewReader(bytes.NewReader(ioData64KB))
			for {
				_, err := reader.Read(chunk)
				if err == io.EOF {
					break
				}
				if err != nil {
					b.Fatal(err)
				}
			}
		}
	})

	b.Run("Writer", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var buf bytes.Buffer
			writer := bufio.NewWriter(&buf)

			// Write 64KB total in 4KB chunks
			for range 16 {
				_, err := writer.Write(ioData4KB)
				if err != nil {
					b.Fatal(err)
				}
			}

			err := writer.Flush()
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
