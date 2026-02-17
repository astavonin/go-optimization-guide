package networking

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// BenchmarkConnectionPool measures HTTP client connection pool efficiency.
// Compares request latency with cold (no pooling) vs warm (with pooling) configurations.
func BenchmarkConnectionPool(b *testing.B) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("OK"))
		if err != nil {
			return
		}
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	b.Run("ColdPool", func(b *testing.B) {
		b.ResetTimer()
		for b.Loop() {
			// Create new transport per iteration (no pooling)
			client := &http.Client{
				Transport: &http.Transport{
					DisableKeepAlives: true,
				},
			}
			resp, err := client.Get(server.URL)
			if err != nil {
				b.Fatal(err)
			}
			_, err = io.Copy(io.Discard, resp.Body)
			if err != nil {
				b.Fatal(err)
			}
			resp.Body.Close()
			client.CloseIdleConnections()
		}
	})

	b.Run("WarmPool", func(b *testing.B) {
		transport := &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		}
		client := &http.Client{Transport: transport}
		defer transport.CloseIdleConnections()

		// Warm up the pool
		for i := 0; i < 10; i++ {
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

		b.ResetTimer()
		for b.Loop() {
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

	b.Run("WarmPool_Parallel", func(b *testing.B) {
		transport := &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		}
		client := &http.Client{Transport: transport}
		defer transport.CloseIdleConnections()

		// Warm up the pool
		for i := 0; i < 10; i++ {
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

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
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
	})
}
