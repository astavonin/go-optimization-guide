package stdlib

import (
	"regexp"
	"testing"
)

// Pre-defined test patterns and input for regexp benchmarks
var (
	regexpPatterns = map[string]string{
		"Email":   `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`,
		"IPv4":    `\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`,
		"LogLine": `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z\s+\[(\w+)\]\s+(.*)$`,
	}

	regexpInput = "2024-01-20T15:30:45.123Z [INFO] Request from user@example.com at 192.168.1.100"

	// Pre-compiled regexps for Match benchmarks
	compiledRegexps map[string]*regexp.Regexp
)

func init() {
	compiledRegexps = make(map[string]*regexp.Regexp)
	for name, pattern := range regexpPatterns {
		compiledRegexps[name] = regexp.MustCompile(pattern)
	}
}

// BenchmarkRegexp measures regular expression compilation and execution performance.
// Baseline for text processing performance across Go versions.
func BenchmarkRegexp(b *testing.B) {
	b.Run("Compile", func(b *testing.B) {
		b.Run("Email", func(b *testing.B) {
			b.ReportAllocs()
			pattern := regexpPatterns["Email"]
			for b.Loop() {
				re, err := regexp.Compile(pattern)
				if err != nil {
					b.Fatal(err)
				}
				_ = re
			}
		})

		b.Run("IPv4", func(b *testing.B) {
			b.ReportAllocs()
			pattern := regexpPatterns["IPv4"]
			for b.Loop() {
				re, err := regexp.Compile(pattern)
				if err != nil {
					b.Fatal(err)
				}
				_ = re
			}
		})

		b.Run("LogLine", func(b *testing.B) {
			b.ReportAllocs()
			pattern := regexpPatterns["LogLine"]
			for b.Loop() {
				re, err := regexp.Compile(pattern)
				if err != nil {
					b.Fatal(err)
				}
				_ = re
			}
		})
	})

	b.Run("Match", func(b *testing.B) {
		b.Run("Email", func(b *testing.B) {
			b.ReportAllocs()
			re := compiledRegexps["Email"]
			for b.Loop() {
				matches := re.FindAllString(regexpInput, -1)
				_ = matches
			}
		})

		b.Run("IPv4", func(b *testing.B) {
			b.ReportAllocs()
			re := compiledRegexps["IPv4"]
			for b.Loop() {
				matches := re.FindAllString(regexpInput, -1)
				_ = matches
			}
		})

		b.Run("LogLine", func(b *testing.B) {
			b.ReportAllocs()
			re := compiledRegexps["LogLine"]
			for b.Loop() {
				matches := re.FindAllString(regexpInput, -1)
				_ = matches
			}
		})
	})
}
