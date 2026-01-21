package networking

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// BenchmarkHTTPRequest measures HTTP request/response cycle time.
// Provides baseline for HTTP/1.1 performance.
func BenchmarkHTTPRequest(b *testing.B) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("OK"))
		if err != nil {
			return
		}
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	client := server.Client()

	b.Run("GET_Small", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			resp, err := client.Get(server.URL)
			if err != nil {
				b.Fatal(err)
			}
			_, err = io.Copy(io.Discard, resp.Body)
			if err != nil {
				b.Fatal(err)
			}
			resp.Body.Close()
		}
	})

	b.Run("POST_1KB", func(b *testing.B) {
		data := bytes.NewReader(make([]byte, 1024))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := data.Seek(0, io.SeekStart)
			if err != nil {
				b.Fatal(err)
			}
			resp, err := client.Post(server.URL, "application/octet-stream", data)
			if err != nil {
				b.Fatal(err)
			}
			_, err = io.Copy(io.Discard, resp.Body)
			if err != nil {
				b.Fatal(err)
			}
			resp.Body.Close()
		}
	})
}

// BenchmarkHTTP2 measures HTTP/2 multiplexing and stream performance.
// Go 1.24: New HTTP2Config API; Go 1.26: StrictMaxConcurrentRequests option.
func BenchmarkHTTP2(b *testing.B) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("OK"))
		if err != nil {
			return
		}
	})

	// Create TLS server with HTTP/2 enabled
	server := httptest.NewUnstartedServer(handler)
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()

	// Configure client with connection limits to force multiplexing
	client := server.Client()
	if transport, ok := client.Transport.(*http.Transport); ok {
		transport.MaxConnsPerHost = 1 // Force connection reuse/multiplexing
	}

	b.Run("Sequential", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			resp, err := client.Get(server.URL)
			if err != nil {
				b.Fatal(err)
			}
			// Verify HTTP/2 was actually negotiated
			if resp.ProtoMajor != 2 {
				b.Fatalf("expected HTTP/2, got HTTP/%d.%d", resp.ProtoMajor, resp.ProtoMinor)
			}
			_, err = io.Copy(io.Discard, resp.Body)
			if err != nil {
				b.Fatal(err)
			}
			resp.Body.Close()
		}
	})

	b.Run("Parallel_10", func(b *testing.B) {
		b.SetParallelism(10)
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				resp, err := client.Get(server.URL)
				if err != nil {
					b.Fatal(err)
				}
				// Verify HTTP/2
				if resp.ProtoMajor != 2 {
					b.Fatalf("expected HTTP/2, got HTTP/%d.%d", resp.ProtoMajor, resp.ProtoMinor)
				}
				_, err = io.Copy(io.Discard, resp.Body)
				if err != nil {
					b.Fatal(err)
				}
				resp.Body.Close()
			}
		})
	})

	b.Run("Parallel_30", func(b *testing.B) {
		b.SetParallelism(30)
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				resp, err := client.Get(server.URL)
				if err != nil {
					b.Fatal(err)
				}
				// Verify HTTP/2
				if resp.ProtoMajor != 2 {
					b.Fatalf("expected HTTP/2, got HTTP/%d.%d", resp.ProtoMajor, resp.ProtoMinor)
				}
				_, err = io.Copy(io.Discard, resp.Body)
				if err != nil {
					b.Fatal(err)
				}
				resp.Body.Close()
			}
		})
	})
}
