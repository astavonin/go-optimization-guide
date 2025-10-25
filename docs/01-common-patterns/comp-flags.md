# Leveraging Compiler Optimization Flags in Go

When tuning Go applications for performance, most of the attention goes to runtime behavior—profiling hot paths, trimming allocations, improving concurrency. But there’s another layer that’s easy to miss: what the Go compiler does with your code before it ever runs. The build process includes several optimization passes, and understanding how to surface or influence them can give you clearer insights into what’s actually happening under the hood. It’s not about tweaking obscure flags to squeeze out extra instructions—it’s about knowing how the compiler treats your code so you’re not working against it.

While Go doesn’t expose the same granular set of compiler flags as C or Rust, it still provides useful ways to influence how your code is built—especially when targeting performance, binary size, or specific environments.

## Why Compiler Flags Matter

Go's compiler (specifically `cmd/compile` and `cmd/link`) performs several default optimizations: inlining, escape analysis, dead code elimination, and more. However, there are scenarios where you can squeeze more performance or control from your build using the right flags.

Use cases include:

- Reducing binary size for minimal containers or embedded systems  
- Building for specific architectures or OSes  
- Removing debug information for release builds  
- Disabling optimizations temporarily for easier debugging  
- Enabling experimental or unsafe performance tricks (carefully)

## Key Compiler and Linker Flags

### `-ldflags="-s -w"` — Strip Debug Info

When you want to shrink binary size, especially in production or containers:

```bash
go build -ldflags="-s -w" -o app main.go
```

- `-s`: Omit the symbol table
- `-w`: Omit DWARF debugging information

Why it matters: This can reduce binary size by up to 30-40%, depending on your codebase. It is useful in Docker images or when distributing binaries.

### `-gcflags` — Control Compiler Optimizations

The `-gcflags` flag allows you to control how the compiler treats specific packages. For example, you can disable optimizations for debugging:

```bash
go build -gcflags="all=-N -l" -o app main.go
```

- `-N`: Disable optimizations
- `-l`: Disable inlining

When to use: During debugging sessions with Delve or similar tools. Turning off inlining and optimizations make stack traces and breakpoints more reliable.

### Cross-Compilation Flags

Need to build for another OS or architecture?

```bash
GOOS=linux GOARCH=arm64 go build -o app main.go
```

- `GOOS`, `GOARCH`: Set target OS and architecture
- Common values: `windows`, `darwin`, `linux`, `amd64`, `arm64`, `386`, `wasm`

### Build Tags

Build tags allow conditional compilation. Use `//go:build` or `// +build` in your source code to control what gets compiled in.

Example:

```go
//go:build debug

package main

import "log"

func debugLog(msg string) {
	log.Println("[DEBUG]", msg)
}
```

Then build with:

```bash
go build -tags=debug -o app main.go
```

### `-ldflags="-X ..."` — Inject Build-Time Variables

You can inject version numbers or metadata into your binary at build time:

```go
// main.go
package main

import "fmt"

var version = "dev"

func main() {
	fmt.Printf("App version: %s\n", version)
}
```

Then build with:

```bash
go build -ldflags="-s -w -X main.version=1.0.0" -o app main.go
```

This sets the `version` variable at link time without modifying your source code. It's useful for embedding release versions, commit hashes, or build dates.

### `-extldflags='-static'` — Build Fully Static Binaries

The `-extldflags '-static'` option passes the `-static` flag to the external system linker, instructing it to produce a **fully statically linked binary**.

This is especially useful when you're using CGO and want to avoid runtime dynamic library dependencies:

```bash
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
CC=gcc \
go build -ldflags="-linkmode=external -extldflags '-static'" -o app main.go
```

What it does:

- Statically links all C libraries into the binary
- Produces a portable, self-contained executable
- Ideal for minimal containers (like `scratch` or `distroless`)

To go further and ensure your binary avoids relying on C library DNS resolution (such as `glibc`'s `getaddrinfo`), you can use the `netgo` build tag. This forces Go to use its pure Go implementation of the DNS resolver:

```bash
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
CC=gcc \
go build -tags netgo -ldflags="-linkmode=external -extldflags '-static'" -o app main.go
```

This step is especially important when building for minimal container environments, where dynamic libc dependencies may not be available.

!!! note
	Static linking requires static versions (`.a`) of the libraries you're using, and may not work with all C libraries by default.

#### Example: Static Build with libcurl via CGO

If you’re using libcurl via CGO, here’s how you can create a statically linked Go binary:

```go
package main

/*
#cgo LDFLAGS: -lcurl
#include <curl/curl.h>
*/
import "C"
import "fmt"

func main() {
	fmt.Println("libcurl version:", C.GoString(C.curl_version()))
}
```

Static Build Command:

```bash
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
CC=gcc \
go build -tags netgo -ldflags="-linkmode=external -extldflags '-static'" -o app main.go
```

Ensure the static version of libcurl (`libcurl.a`) is available on your system. You may need to install development packages or build libcurl from source with `--enable-static`.
