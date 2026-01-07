# Zero-Copy Techniques

When writing performance-critical Go code, how memory is managed often has a bigger impact than it first appears. Zero-copy techniques are one of the more effective ways to tighten that control. Instead of moving bytes from buffer to buffer, these techniques work directly on existing memory—avoiding copies altogether. That means less pressure on the CPU, better cache behavior, and fewer GC-triggered pauses. For I/O-heavy systems—whether you’re streaming files, handling network traffic, or parsing large datasets—this can translate into much higher throughput and lower latency without adding complexity.

## Understanding Zero-Copy

In the usual I/O path, data moves back and forth between user space and kernel space—first copied into a kernel buffer, then into your application’s buffer, or the other way around. It works, but it’s wasteful. Every copy burns CPU cycles and clogs up memory bandwidth. Zero-copy changes that. Instead of bouncing data between buffers, it lets applications work directly with what’s already in place—no detours, no extra copies. The result? Lower CPU load, better use of memory, and faster I/O, especially when throughput or latency actually matter.

## Common Zero-Copy Techniques in Go

### Using `io.Reader` and `io.Writer` Interfaces

Using interfaces like `io.Reader` and `io.Writer` gives you fine-grained control over how data flows. Instead of spinning up new buffers every time, you can reuse existing ones and keep memory usage steady. In practice, this avoids unnecessary garbage collection pressure and keeps your I/O paths clean and efficient—especially when you’re dealing with high-throughput or streaming workloads.

```go
func StreamData(src io.Reader, dst io.Writer) error {
	buf := make([]byte, 4096) // Reusable buffer
	_, err := io.CopyBuffer(dst, src, buf)
	return err
}
```

`io.CopyBuffer` reuses a provided buffer, avoiding repeated allocations and intermediate copies. An in-depth `io.CopyBuffer` explanation is [available on SO](https://stackoverflow.com/questions/71082021/what-exactly-is-buffer-last-parameter-in-io-copybuffer).

### Slicing for Efficient Data Access

Slicing large byte arrays or buffers instead of copying data into new slices is a powerful zero-copy strategy:

```go
func process(buffer []byte) []byte {
	return buffer[128:256] // returns a slice reference without copying
}
```

Slices in Go are inherently zero-copy since they reference the underlying array.

### Benchmarking Impact

Here's a basic benchmark illustrating performance differences between explicit copying and zero-copy slicing:


```go
{%
    include-markdown "01-common-patterns/src/zero-copy_test.go"
    start="// bench-start"
    end="// bench-end"
%}
```

In `BenchmarkCopy`, each iteration copies a 64KB buffer into a fresh slice—allocating memory and duplicating data every time. That cost adds up fast. `BenchmarkSlice`, on the other hand, just re-slices the same buffer—no allocation, no copying, just new view on the same data. The difference is night and day. When performance matters, avoiding copies isn’t just a micro-optimization—it’s fundamental.

!!! info
	These two functions are not equivalent in behavior—`BenchmarkCopy` makes an actual deep copy of the buffer, while `BenchmarkSlice` only creates a new slice header pointing to the same underlying data. This benchmark is not comparing functional correctness but is intentionally contrasting performance characteristics to highlight the cost of unnecessary copying.

| Benchmark                | Time per op (ns) | Bytes per op | Allocs per op |
|--------------------------|---------|--------|------------|
| BenchmarkCopy            | 4,246   | 65536 | 1          |
| BenchmarkSlice           | 0.592   | 0     | 0          |


## Memory Mapping (`mmap`): Syscall Avoidance vs. Zero-Copy

Memory mapping is often described as a zero-copy technique, but that description only holds for a specific access pattern (see [Issue#25](https://github.com/astavonin/go-optimization-guide/issues/25) for more details). Mapping a file does not, by itself, eliminate copying. Zero-copy only occurs when the application operates directly on the mapped pages. If data is copied out into another buffer, the copy cost remains, regardless of how the file was opened.

This section separates two effects that are often conflated: avoiding per-iteration syscalls and avoiding memory copies.

## `mmap` with `Copy`: Avoiding Syscalls, Not Copies

The first comparison uses `os.ReadAt` versus `golang.org/x/exp/mmap` with `ReadAt`:

```go
{%
    include-markdown "01-common-patterns/src/zero-copy_test.go"
    start="// bench-io-start"
    end="// bench-io-end"
%}
```

Both benchmarks copy 4MB of data into a user-space buffer on every iteration. The difference is where the copy is initiated.

* `ReadAt` performs a system call on each iteration and copies data from the kernel page cache into user memory.
* `mmap.ReadAt` still copies data into a user buffer, but avoids the per-iteration syscall by reading from already-mapped pages.

!!! warning
	This benchmark does **not** measure zero-copy behavior. It measures syscall and context-switch overhead.

### Benchmarking Impact

| Benchmark    | Time per op (ns) | Syscalls per iter |
| ------------ | ---------------- | ---------------  |
| BenchmarkReadWithCopy | 241,354     | 1 (`pread`)       |
| BenchmarkReadWithMmap | 181,191     | 0                 |

The performance difference comes from syscall avoidance, not from reduced memory movement. It is also worth noting that `golang.org/x/exp/mmap` does not expose the mapped memory directly. As long as access goes through `ReadAt`, a copy is unavoidable.

## True Zero-Copy with `unix.Mmap`

Memory mapping becomes zero-copy only when the application operates directly on the mapped pages. To demonstrate this, the next benchmarks use `unix.Mmap` and consume the mapped memory without copying it into a separate buffer. To ensure the mapped data is actually read and not optimized away, each iteration processes a fixed 4MB window.

### Memory-Bound Workload (`XXHash`)

In the copy-based version, each iteration performs:

* a kernel-to-user memory copy
* hashing over the copied buffer

```go
{%
    include-markdown "01-common-patterns/src/zero-copy_test.go"
    start="// bench-hash-start"
    end="// bench-hash-end"
%}
```

In the mmap version:

* file-backed pages are accessed directly
* no additional user-space copy occurs

```go
{%
    include-markdown "01-common-patterns/src/zero-copy_test.go"
    start="// bench-hash-mmap-start"
    end="// bench-hash-mmap-end"
%}
```

XXHash is used here because it is lightweight enough that memory movement and cache behavior remain visible in the measurements.

#### Benchmarking Impact

| Benchmark        | Time per op (ns) | Copies per iter | Dominant cost       |
| ---------------- | ---------------- | --------------- | ------------------- |
| ReadAtCopyXXHash | 539,512         | 1 (4MB)         | Copy + hash         |
| MmapNoCopyXXHash | 281,249         | 0               | Hash + memory reads |

The roughly 2× difference reflects the removal of a full 4MB memory copy from the critical path. This is an actual zero-copy scenario.

### Compute-Dominated Workload (SHA256)

These tests are identical to XXHash-based tests but use a different approach to simulate a more compute-intensive workflow via SHA calculations.

BenchmarkReadAtCopySHA:

```go
{%
    include-markdown "01-common-patterns/src/zero-copy_test.go"
    start="// bench-sha-start"
    end="// bench-sha-end"
%}
```

BenchmarkMmapNoCopySHA:

```go
{%
    include-markdown "01-common-patterns/src/zero-copy_test.go"
    start="// bench-sha-mmap-start"
    end="// bench-sha-mmap-end"
%}
```

#### Benchmarking Impact

| Benchmark     | Time per op (ns) | Copies per iter | Dominant cost      |
| ------------- | ---------------- | --------------- | ------------------ |
| ReadAtCopySHA | 2,636,956       | 1 (4MB)         | SHA256 computation |
| MmapNoCopySHA | 2,287,858       | 0               | SHA256 computation |

The `mmap` version remains faster, but the difference is smaller. Once the CPU is dominated by cryptographic computation, eliminating a memory copy has a reduced impact on total runtime. The remaining improvement comes from lower memory bandwidth pressure and cache effects.

### Summary Comparison

| Scenario                      | Zero-Copy | Primary Effect         | Observed Impact         |
| ----------------------------- | --------- | ---------------------- | ----------------------- |
| mmap + `ReadAt`               | No        | Syscall avoidance      | Moderate improvement    |
| mmap + direct access          | Yes       | Copy elimination       | Large (memory-bound)    |
| mmap + direct access + SHA256 | Yes       | Reduced memory traffic | Limited (compute-bound) |

??? example "Show the complete benchmark file"
    ```go
    {% include "01-common-patterns/src/interface-boxing_test.go" %}
    ```

??? info "How to run the benchmark"
	To run the benchmark involving `mmap`, you’ll need to install the required package and create a test file:

	```bash
	go get golang.org/x/exp/mmap
	go get golang.org/x/sys/unix
	mkdir -p testdata
	dd if=/dev/urandom of=./testdata/largefile.bin bs=1M count=4
	```


## When to Use Zero-Copy

:material-checkbox-marked-circle-outline: Zero-copy techniques are highly beneficial for:

- Network servers handling large amounts of concurrent data streams. Avoiding unnecessary memory copies helps reduce CPU usage and latency, especially under high load.
- Applications with heavy I/O operations like file streaming or real-time data processing. Zero-copy allows data to move through the system efficiently without redundant allocations or copies.

!!! warning
	:fontawesome-regular-hand-point-right: Zero-copy isn’t a free win. Slices share underlying memory, so reusing them means you’re also sharing state. If one part of your code changes the data while another is still reading it, you’re setting yourself up for subtle, hard-to-track bugs. This kind of shared memory requires discipline—clear ownership and tight control. It also adds complexity, which might not be worth it unless the performance gains are real and measurable. Always benchmark before committing to it.

### Real-World Use Cases and Libraries

Zero-copy strategies aren't just theoretical—they're used in production by performance-critical Go systems:

- [fasthttp](https://github.com/valyala/fasthttp): A high-performance HTTP server designed to avoid allocations. It returns slices directly and avoids `string` conversions to minimize copying.
- [gRPC-Go](https://github.com/grpc/grpc-go): Uses internal buffer pools and avoids deep copying of large request/response messages to reduce GC pressure.
- [MinIO](https://github.com/minio/minio): An object storage system that streams data directly between disk and network using `io.Reader` without unnecessary buffer replication.
- [Protobuf](https://github.com/protocolbuffers/protobuf) and [MsgPack](https://github.com/vmihailenco/msgpack) libraries: Efficient serialization frameworks like `google.golang.org/protobuf` and `vmihailenco/msgpack` support decoding directly into user-managed buffers.
- [InfluxDB](https://github.com/influxdata/influxdb) and [Badger](https://github.com/hypermodeinc/badger): These storage engines use `mmap` extensively for fast, zero-copy access to database files.

These libraries show how zero-copy techniques help reduce allocations, GC overhead, and system call frequency—all while increasing throughput.
