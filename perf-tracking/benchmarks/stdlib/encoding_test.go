package stdlib

import (
	"encoding/binary"
	"encoding/json"
	"testing"
)

// APIResponse represents realistic API payload structure for JSON benchmarks.
type APIResponse struct {
	ID        int64          `json:"id"`
	Name      string         `json:"name"`
	Email     string         `json:"email"`
	Tags      []string       `json:"tags"`
	Metadata  map[string]any `json:"metadata"`
	CreatedAt string         `json:"created_at"`
	Active    bool           `json:"active"`
}

// BinaryData represents fixed-size struct for binary encoding benchmarks.
type BinaryData struct {
	A uint64
	B uint32
	C uint16
	D uint8
}

// Pre-generated deterministic JSON payloads
var (
	jsonSmall = []byte(`{"id":1,"name":"Test","email":"test@example.com","tags":[],"metadata":{},"created_at":"2024-01-20T12:00:00Z","active":true}`)

	jsonMedium = []byte(`{"id":12345,"name":"Test User","email":"user@example.com","tags":["go","performance","benchmark"],"metadata":{"score":95.5,"verified":true,"level":"premium"},"created_at":"2024-01-20T12:00:00Z","active":true}`)

	jsonLarge = []byte(`{"id":123456789,"name":"Extended Test User Profile","email":"extended.user@example.com","tags":["go","performance","benchmark","optimization","stdlib","testing","validation","production"],"metadata":{"score":95.5,"verified":true,"level":"premium","tier":"enterprise","region":"us-west","datacenter":"pdx-1","version":"2.1.0","features":["api","websocket","graphql"],"limits":{"rate":1000,"burst":100,"concurrent":50},"timestamps":{"created":"2024-01-01T00:00:00Z","updated":"2024-01-20T12:00:00Z","expires":"2025-01-20T12:00:00Z"},"contact":{"phone":"+1-555-0100","address":"123 Main St","city":"Portland","state":"OR","zip":"97201"},"preferences":{"notifications":true,"marketing":false,"analytics":true}},"created_at":"2024-01-20T12:00:00Z","active":true}`)

	// Pre-allocated struct for encoding
	encodeSmall = APIResponse{
		ID:        1,
		Name:      "Test",
		Email:     "test@example.com",
		Tags:      []string{},
		Metadata:  map[string]any{},
		CreatedAt: "2024-01-20T12:00:00Z",
		Active:    true,
	}

	encodeWithEscaping = APIResponse{
		ID:        12345,
		Name:      "Test \"User\" with\nescapes\t",
		Email:     "user+special@example.com",
		Tags:      []string{"tag/with/slashes", "tag\"with\"quotes"},
		Metadata:  map[string]any{"key": "value with \"quotes\" and \n newlines"},
		CreatedAt: "2024-01-20T12:00:00Z",
		Active:    true,
	}
)

// BenchmarkJSONDecode measures JSON decoding performance into typed struct.
// Go 1.25+ with GOEXPERIMENT=jsonv2 shows substantial improvement.
func BenchmarkJSONDecode(b *testing.B) {
	b.Run("Small", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			var resp APIResponse
			err := json.Unmarshal(jsonSmall, &resp)
			if err != nil {
				b.Fatal(err)
			}
			_ = resp
		}
	})

	b.Run("Medium", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			var resp APIResponse
			err := json.Unmarshal(jsonMedium, &resp)
			if err != nil {
				b.Fatal(err)
			}
			_ = resp
		}
	})

	b.Run("Large", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			var resp APIResponse
			err := json.Unmarshal(jsonLarge, &resp)
			if err != nil {
				b.Fatal(err)
			}
			_ = resp
		}
	})
}

// BenchmarkJSONEncode measures JSON encoding performance from typed struct.
// JSON v2 improves encoding as well as decoding.
func BenchmarkJSONEncode(b *testing.B) {
	b.Run("Small", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			data, err := json.Marshal(encodeSmall)
			if err != nil {
				b.Fatal(err)
			}
			_ = data
		}
	})

	b.Run("WithEscaping", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			data, err := json.Marshal(encodeWithEscaping)
			if err != nil {
				b.Fatal(err)
			}
			_ = data
		}
	})
}

// BenchmarkBinaryEncode measures binary encoding performance with Go 1.23+ APIs.
// New binary.Encode/Append APIs avoid reflection overhead of binary.Write.
func BenchmarkBinaryEncode(b *testing.B) {
	data := BinaryData{
		A: 0xDEADBEEFCAFEBABE,
		B: 0xCAFEBABE,
		C: 0xBEEF,
		D: 0xFF,
	}

	b.Run("Encode", func(b *testing.B) {
		buf := make([]byte, binary.Size(data))

		for i := 0; i < b.N; i++ {
			_, err := binary.Encode(buf, binary.LittleEndian, data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Append", func(b *testing.B) {
		buf := make([]byte, 0, binary.Size(data))

		for i := 0; i < b.N; i++ {
			result, err := binary.Append(buf[:0], binary.LittleEndian, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})

	b.Run("LegacyWrite", func(b *testing.B) {
		buf := make([]byte, binary.Size(data))

		for i := 0; i < b.N; i++ {
			w := &bytesWriter{buf: buf[:0]}
			err := binary.Write(w, binary.LittleEndian, data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// bytesWriter implements io.Writer for binary.Write benchmark.
type bytesWriter struct {
	buf []byte
}

func (w *bytesWriter) Write(p []byte) (n int, err error) {
	w.buf = append(w.buf, p...)
	return len(p), nil
}
