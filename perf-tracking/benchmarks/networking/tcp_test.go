package networking

import (
	"io"
	"net"
	"testing"
)

// BenchmarkTCPConnect measures TCP connection establishment time to localhost.
// Provides baseline for connection handling performance.
func BenchmarkTCPConnect(b *testing.B) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	defer ln.Close()

	// Accept loop to drain connections
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	addr := ln.Addr().String()

	b.Run("Sequential", func(b *testing.B) {
		b.ResetTimer()
		for b.Loop() {
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				b.Fatal(err)
			}
			// SetLinger(0) sends RST on close, avoiding TIME_WAIT buildup
			// that exhausts ephemeral ports on macOS during high-iteration runs.
			conn.(*net.TCPConn).SetLinger(0)
			conn.Close()
		}
	})

	b.Run("Parallel", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				conn, err := net.Dial("tcp", addr)
				if err != nil {
					b.Fatal(err)
				}
				conn.(*net.TCPConn).SetLinger(0)
				conn.Close()
			}
		})
	})
}

// BenchmarkTCPKeepAlive measures TCP keep-alive configuration overhead.
// Go 1.23+ provides KeepAliveConfig API for fine-grained control.
func BenchmarkTCPKeepAlive(b *testing.B) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	defer ln.Close()

	// Accept loop to drain connections
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	addr := ln.Addr().String()

	b.Run("Default", func(b *testing.B) {
		dialer := &net.Dialer{}
		b.ResetTimer()
		for b.Loop() {
			conn, err := dialer.Dial("tcp", addr)
			if err != nil {
				b.Fatal(err)
			}
			conn.(*net.TCPConn).SetLinger(0)
			conn.Close()
		}
	})

	b.Run("WithConfig", func(b *testing.B) {
		dialer := &net.Dialer{
			KeepAlive: 30000000000, // 30 seconds in nanoseconds
		}
		b.ResetTimer()
		for b.Loop() {
			conn, err := dialer.Dial("tcp", addr)
			if err != nil {
				b.Fatal(err)
			}
			conn.(*net.TCPConn).SetLinger(0)
			conn.Close()
		}
	})
}

// BenchmarkTCPThroughput measures TCP data transfer throughput with various buffer sizes.
// Provides baseline for comparing TLS and HTTP overhead.
func BenchmarkTCPThroughput(b *testing.B) {
	sizes := []struct {
		name string
		size int
	}{
		{"1KB", 1024},
		{"64KB", 64 * 1024},
		{"1MB", 1024 * 1024},
	}

	for _, s := range sizes {
		b.Run(s.name, func(b *testing.B) {
			// Setup echo server
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				b.Fatal(err)
			}
			defer ln.Close()

			// Echo server goroutine
			go func() {
				for {
					conn, err := ln.Accept()
					if err != nil {
						return
					}
					go func(c net.Conn) {
						defer c.Close()
						io.Copy(c, c)
					}(conn)
				}
			}()

			// Connect to server
			conn, err := net.Dial("tcp", ln.Addr().String())
			if err != nil {
				b.Fatal(err)
			}
			defer conn.Close()

			// Prepare test data
			data := make([]byte, s.size)
			for i := range data {
				data[i] = byte(i % 256)
			}
			buf := make([]byte, s.size)

			b.SetBytes(int64(2 * s.size))
			b.ResetTimer()

			for b.Loop() {
				_, err := conn.Write(data)
				if err != nil {
					b.Fatal(err)
				}
				_, err = io.ReadFull(conn, buf)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
