package main

import (
	"fmt"
	"os"
)

// benchcompare - Compare benchmark results between Go versions
//
// Usage:
//   benchcompare -baseline <file> -target <file> [-output <file>]

func main() {
	fmt.Println("benchcompare - Go version performance comparison tool")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  benchcompare -baseline <baseline.json> -target <target.json> [-output <report.md>]")
	fmt.Println()
	fmt.Println("Not yet implemented")
	os.Exit(0)
}
