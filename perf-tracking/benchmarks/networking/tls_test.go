package networking

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"testing"
	"time"
)

var (
	tlsTestCert tls.Certificate
)

func init() {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(fmt.Sprintf("failed to generate private key: %v", err))
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Benchmark Test"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		panic(fmt.Sprintf("failed to create certificate: %v", err))
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal private key: %v", err))
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	tlsTestCert, err = tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(fmt.Sprintf("failed to load X509 key pair: %v", err))
	}
}

// BenchmarkTLSHandshake measures TLS handshake time with various configurations.
// Go 1.24: X25519MLKEM768 default; Go 1.25: SHA-1 disabled; Go 1.26: Post-quantum default.
func BenchmarkTLSHandshake(b *testing.B) {
	serverConfig := &tls.Config{
		Certificates: []tls.Certificate{tlsTestCert},
		MinVersion:   tls.VersionTLS12,
	}

	ln, err := tls.Listen("tcp", "127.0.0.1:0", serverConfig)
	if err != nil {
		b.Fatal(err)
	}
	defer ln.Close()

	// Accept loop to complete TLS handshakes
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				if tlsConn, ok := c.(*tls.Conn); ok {
					if err := tlsConn.Handshake(); err != nil {
						// Server-side handshake errors expected when client disconnects early
						return
					}
				}
			}(conn)
		}
	}()

	addr := ln.Addr().String()

	b.Run("TLS12", func(b *testing.B) {
		clientConfig := &tls.Config{
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS12,
			MaxVersion:         tls.VersionTLS12,
		}
		b.ResetTimer()
		for b.Loop() {
			conn, err := tls.Dial("tcp", addr, clientConfig)
			if err != nil {
				b.Fatal(err)
			}
			conn.Close()
		}
	})

	b.Run("TLS13", func(b *testing.B) {
		clientConfig := &tls.Config{
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS13,
		}
		b.ResetTimer()
		for b.Loop() {
			conn, err := tls.Dial("tcp", addr, clientConfig)
			if err != nil {
				b.Fatal(err)
			}
			conn.Close()
		}
	})

	b.Run("TLS13_ECDHE", func(b *testing.B) {
		clientConfig := &tls.Config{
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS13,
			CurvePreferences:   []tls.CurveID{tls.X25519},
		}
		b.ResetTimer()
		for b.Loop() {
			conn, err := tls.Dial("tcp", addr, clientConfig)
			if err != nil {
				b.Fatal(err)
			}
			conn.Close()
		}
	})
}

// BenchmarkTLSResume measures TLS session resumption performance.
// Compares full handshake vs resumed handshake (tickets/PSK).
func BenchmarkTLSResume(b *testing.B) {
	serverConfig := &tls.Config{
		Certificates:           []tls.Certificate{tlsTestCert},
		MinVersion:             tls.VersionTLS13,
		SessionTicketsDisabled: false, // Explicitly enable session tickets
	}

	ln, err := tls.Listen("tcp", "127.0.0.1:0", serverConfig)
	if err != nil {
		b.Fatal(err)
	}
	defer ln.Close()

	// Accept loop to complete TLS handshakes
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				if tlsConn, ok := c.(*tls.Conn); ok {
					if err := tlsConn.Handshake(); err != nil {
						// Server-side handshake errors expected when client disconnects early
						return
					}
				}
			}(conn)
		}
	}()

	addr := ln.Addr().String()

	b.Run("FullHandshake", func(b *testing.B) {
		clientConfig := &tls.Config{
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS13,
		}
		b.ResetTimer()
		for b.Loop() {
			conn, err := tls.Dial("tcp", addr, clientConfig)
			if err != nil {
				b.Fatal(err)
			}
			state := conn.ConnectionState()
			if state.DidResume {
				b.Fatal("expected full handshake, got resumption")
			}
			conn.Close()
		}
	})

	b.Run("Resumed", func(b *testing.B) {
		cache := tls.NewLRUClientSessionCache(1)
		clientConfig := &tls.Config{
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS13,
			ClientSessionCache: cache,
		}

		// Establish multiple initial connections to populate cache
		// TLS 1.3 sends session tickets after handshake
		for i := 0; i < 3; i++ {
			conn, err := tls.Dial("tcp", addr, clientConfig)
			if err != nil {
				b.Fatal(err)
			}
			// Force handshake and wait for session ticket
			err = conn.Handshake()
			if err != nil {
				b.Fatal(err)
			}
			time.Sleep(50 * time.Millisecond)
			conn.Close()
		}

		// Verify resumption works
		testConn, err := tls.Dial("tcp", addr, clientConfig)
		if err != nil {
			b.Fatal(err)
		}
		testState := testConn.ConnectionState()
		testConn.Close()

		if !testState.DidResume {
			b.Skip("session resumption not working, skipping resumed benchmark")
		}

		resumedCount := 0
		var n int
		b.ResetTimer()
		for b.Loop() {
			conn, err := tls.Dial("tcp", addr, clientConfig)
			if err != nil {
				b.Fatal(err)
			}
			if conn.ConnectionState().DidResume {
				resumedCount++
			}
			conn.Close()
			n++
		}
		b.ReportMetric(100.0*float64(resumedCount)/float64(n), "resumed-%")
	})
}

// BenchmarkTLSThroughput measures TLS encrypted data transfer throughput.
// Shows overhead of encryption vs plain TCP.
func BenchmarkTLSThroughput(b *testing.B) {
	serverConfig := &tls.Config{
		Certificates: []tls.Certificate{tlsTestCert},
		MinVersion:   tls.VersionTLS13,
	}

	ln, err := tls.Listen("tcp", "127.0.0.1:0", serverConfig)
	if err != nil {
		b.Fatal(err)
	}
	defer ln.Close()

	// TLS echo server
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

	sizes := []struct {
		name string
		size int
	}{
		{"1KB", 1024},
		{"64KB", 64 * 1024},
	}

	for _, s := range sizes {
		b.Run(s.name, func(b *testing.B) {
			clientConfig := &tls.Config{
				InsecureSkipVerify: true,
				MinVersion:         tls.VersionTLS13,
			}

			conn, err := tls.Dial("tcp", ln.Addr().String(), clientConfig)
			if err != nil {
				b.Fatal(err)
			}
			defer conn.Close()

			data := make([]byte, s.size)
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
