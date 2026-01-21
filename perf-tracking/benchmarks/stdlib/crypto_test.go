package stdlib

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"testing"

	"golang.org/x/crypto/sha3"
)

// Pre-generated deterministic crypto test data
var (
	cryptoKey32   []byte // AES-256 key
	cryptoIV      []byte // AES IV
	cryptoNonce12 []byte // GCM nonce
	cryptoData1KB []byte
	cryptoData64KB []byte
	cryptoData1MB  []byte
	cryptoData64B  []byte
	cryptoData16KB []byte
)

func init() {
	// Generate deterministic crypto keys and IVs
	cryptoKey32 = make([]byte, 32) // AES-256
	for i := range cryptoKey32 {
		cryptoKey32[i] = byte(i)
	}

	cryptoIV = make([]byte, aes.BlockSize)
	for i := range cryptoIV {
		cryptoIV[i] = byte(i + 100)
	}

	cryptoNonce12 = make([]byte, 12) // Standard GCM nonce size
	for i := range cryptoNonce12 {
		cryptoNonce12[i] = byte(i + 50)
	}

	// Generate deterministic crypto data
	cryptoData64B = make([]byte, 64)
	for i := range cryptoData64B {
		cryptoData64B[i] = byte(i % 256)
	}

	cryptoData1KB = make([]byte, 1024)
	for i := range cryptoData1KB {
		cryptoData1KB[i] = byte(i % 256)
	}

	cryptoData16KB = make([]byte, 16*1024)
	for i := range cryptoData16KB {
		cryptoData16KB[i] = byte(i % 256)
	}

	cryptoData64KB = make([]byte, 64*1024)
	for i := range cryptoData64KB {
		cryptoData64KB[i] = byte(i % 256)
	}

	cryptoData1MB = make([]byte, 1024*1024)
	for i := range cryptoData1MB {
		cryptoData1MB[i] = byte(i % 256)
	}
}

// BenchmarkAESCTR measures AES-CTR encryption/decryption performance.
// Go 1.24+ shows several times faster performance on amd64/arm64 with hardware acceleration.
func BenchmarkAESCTR(b *testing.B) {
	block, err := aes.NewCipher(cryptoKey32)
	if err != nil {
		b.Fatal(err)
	}

	sizes := []struct {
		name string
		data []byte
	}{
		{"Size1KB", cryptoData1KB},
		{"Size64KB", cryptoData64KB},
		{"Size1MB", cryptoData1MB},
	}

	for _, tc := range sizes {
		b.Run(tc.name, func(b *testing.B) {
			ciphertext := make([]byte, len(tc.data))
			b.SetBytes(int64(len(tc.data)))

			for i := 0; i < b.N; i++ {
				// Reset stream for each iteration to avoid state carryover
				stream := cipher.NewCTR(block, cryptoIV)
				stream.XORKeyStream(ciphertext, tc.data)
			}
		})
	}
}

// BenchmarkSHA measures SHA hashing performance across variants.
// SHA-1: 2x faster with SHA-NI on amd64 (Go 1.26).
// SHA-3: 2x faster on Apple M processors (Go 1.26).
func BenchmarkSHA(b *testing.B) {
	b.Run("SHA1", func(b *testing.B) {
		b.SetBytes(int64(len(cryptoData64KB)))
		for i := 0; i < b.N; i++ {
			h := sha1.Sum(cryptoData64KB)
			_ = h
		}
	})

	b.Run("SHA256", func(b *testing.B) {
		b.SetBytes(int64(len(cryptoData64KB)))
		for i := 0; i < b.N; i++ {
			h := sha256.Sum256(cryptoData64KB)
			_ = h
		}
	})

	b.Run("SHA512", func(b *testing.B) {
		b.SetBytes(int64(len(cryptoData64KB)))
		for i := 0; i < b.N; i++ {
			h := sha512.Sum512(cryptoData64KB)
			_ = h
		}
	})

	b.Run("SHA3_256", func(b *testing.B) {
		h := sha3.New256()
		b.SetBytes(int64(len(cryptoData64KB)))

		for i := 0; i < b.N; i++ {
			h.Reset()
			h.Write(cryptoData64KB)
			sum := h.Sum(nil)
			_ = sum
		}
	})
}

// BenchmarkRSAKeyGen measures RSA key generation performance.
// Go 1.26 shows ~3x faster key generation with improved algorithm.
func BenchmarkRSAKeyGen(b *testing.B) {
	b.Run("Bits2048", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := rsa.GenerateKey(rand.Reader, 2048)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Bits4096", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := rsa.GenerateKey(rand.Reader, 4096)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkAESGCM measures AES-GCM authenticated encryption performance.
// AES-GCM is the dominant cipher in TLS - baseline for encrypted communication performance.
func BenchmarkAESGCM(b *testing.B) {
	block, err := aes.NewCipher(cryptoKey32)
	if err != nil {
		b.Fatal(err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		b.Fatal(err)
	}

	sizes := []struct {
		name string
		data []byte
	}{
		{"Size64", cryptoData64B},
		{"Size1KB", cryptoData1KB},
		{"Size16KB", cryptoData16KB},
	}

	for _, tc := range sizes {
		b.Run(tc.name, func(b *testing.B) {
			ciphertext := make([]byte, 0, len(tc.data)+aead.Overhead())
			b.SetBytes(int64(len(tc.data)))

			for i := 0; i < b.N; i++ {
				// Reuse ciphertext buffer
				ciphertext = aead.Seal(ciphertext[:0], cryptoNonce12, tc.data, nil)
				_ = ciphertext
			}
		})
	}
}
