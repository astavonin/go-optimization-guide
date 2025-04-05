package perf

import (
	"bufio"
	"io"
	"os"
	"strconv"
	"sync"
	"testing"
)

type Data struct {
	Value []byte
}

var dataPool = sync.Pool{
	New: func() any {
		return &Data{Value: make([]byte, 0, 32)}
	},
}

const N = 10000

func writeNotBuffered(w io.Writer, count int) {
	for i := 0; i < count; i++ {
		d := dataPool.Get().(*Data)
		d.Value = strconv.AppendInt(d.Value[:0], int64(i), 10)
		w.Write(d.Value)
		w.Write([]byte(":val\n"))
		dataPool.Put(d)
	}
}

func writeBuffered(w io.Writer, count int) {
	buf := bufio.NewWriterSize(w, 16*1024)
	for i := 0; i < count; i++ {
		d := dataPool.Get().(*Data)
		d.Value = strconv.AppendInt(d.Value[:0], int64(i), 10)
		buf.Write(d.Value)
		buf.Write([]byte(":val\n"))
		dataPool.Put(d)
	}
	buf.Flush()
}

func BenchmarkWriteNotBuffered(b *testing.B) {
	for i := 0; i < b.N; i++ {
		f, _ := os.CreateTemp("", "nobuf")
		writeNotBuffered(f, N)
		f.Close()
		os.Remove(f.Name())
	}
}

func BenchmarkWriteBuffered(b *testing.B) {
	for i := 0; i < b.N; i++ {
		f, _ := os.CreateTemp("", "buf")
		writeBuffered(f, N)
		f.Close()
		os.Remove(f.Name())
	}
}
