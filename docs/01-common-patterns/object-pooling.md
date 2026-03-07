# Object Pooling

Object pooling helps reduce allocation churn in high-throughput Go programs by reusing objects instead of allocating fresh ones each time. This avoids repeated work for the allocator and eases pressure on the garbage collector, especially when dealing with short-lived or frequently reused structures.

Go’s `sync.Pool` provides a built-in way to implement pooling with minimal code. It’s particularly effective for objects that are expensive to allocate or that would otherwise contribute to frequent garbage collection cycles. While not a silver bullet, it’s a low-friction tool that can lead to noticeable gains in latency and CPU efficiency under sustained load.

## How Object Pooling Works

Object pooling allows programs to reuse memory by recycling previously allocated objects instead of creating new ones on every use. Rather than hitting the heap each time, objects are retrieved from a shared pool and returned once they’re no longer needed. This reduces the number of allocations, cuts down on garbage collection workload, and leads to more predictable performance—especially in workloads with high object churn or tight latency requirements.

### Using `sync.Pool` for Object Reuse

`bytes.Buffer` is one of the most common pooling targets in Go—used internally by `net/http`, `encoding/json`, and `fmt`. Each handler or encoder needs a scratch buffer for the duration of a request, then discards it. Without pooling, that means a heap allocation on every call.

#### Without Object Pooling

```go
package main

import (
    "bytes"
    "fmt"
)

func handleRequest(payload []byte) {
    buf := &bytes.Buffer{} // new backing array allocated on every call
    buf.Write(payload)
    fmt.Println(buf.Len())
}
```

Every call to `handleRequest` allocates a fresh backing array for the buffer. Under load—thousands of requests per second—this creates constant allocation churn and keeps the GC busy reclaiming short-lived memory.

#### With Object Pooling

```go
package main

import (
    "bytes"
    "fmt"
    "sync"
)

var bufPool = sync.Pool{
    New: func() any {
        return new(bytes.Buffer)
    },
}

func handleRequest(payload []byte) {
    buf := bufPool.Get().(*bytes.Buffer)
    buf.Reset() // reposition read offset; backing array is kept
    buf.Write(payload)
    fmt.Println(buf.Len())
    bufPool.Put(buf)
}
```

`Reset()` sets the buffer's internal offset to zero without freeing the underlying slice. On the next `Get()`, the buffer arrives already sized from its previous use—`Write` fills existing memory and no allocation occurs.

## Benchmarking Impact

The benchmark writes a 4 KB payload into a `bytes.Buffer` on each iteration, simulating per-request serialization work.

??? example "Show the benchmark file"
    ```go
    {% include "01-common-patterns/src/object-pooling_test.go" %}
    ```

| Benchmark               | Iterations  | Time per op (ns) | Bytes per op | Allocs per op |
|-------------------------|-------------|------------------|---------------|----------------|
| BenchmarkWithoutPooling | 1,328,937   | 864              | 4,096         | 1              |
| BenchmarkWithPooling    | 28,021,245  | 42               | 0             | 0              |

Without pooling, every iteration allocates a fresh 4 KB backing array for the buffer—one heap allocation per call, with the allocator and GC paying the cost. With pooling, the buffer is retrieved from the pool already sized from a prior use: `Reset()` repositions the read offset without freeing the underlying slice, so subsequent writes reuse the existing memory with zero allocations. The result is roughly a 20× throughput improvement and complete elimination of per-call allocation pressure—which directly translates to reduced GC pause frequency at scale.

## When Should You Use `sync.Pool`?

:material-checkbox-marked-circle-outline: Use sync.Pool when:

- You have short-lived, reusable objects (e.g., buffers, scratch memory, request state). Pooling avoids repeated allocations and lets you recycle memory efficiently.
- Allocation overhead or GC churn is measurable and significant. Reusing objects reduces the number of heap allocations, which in turn lowers garbage collection frequency and pause times.
- The object’s lifecycle is local and can be reset between uses. When objects don’t need complex teardown and are safe to reuse after a simple reset, pooling is straightforward and effective.
- You want to reduce pressure on the garbage collector in high-throughput systems. In systems handling thousands of requests per second, pooling helps maintain consistent performance and minimizes GC-related latency spikes.

:fontawesome-regular-hand-point-right: Avoid sync.Pool when:

- Objects are long-lived or shared across multiple goroutines. `sync.Pool` is optimized for short-lived, single-use objects and doesn’t manage shared ownership or coordination.
- The reuse rate is low and pooled objects are not frequently accessed. If objects sit idle in the pool, you gain little benefit and may even waste memory.
- Predictability or lifecycle control is more important than allocation speed. Pooling makes lifecycle tracking harder and may not be worth the tradeoff.
- Memory savings are negligible or code complexity increases significantly. If pooling doesn’t provide clear benefits, it can add unnecessary complexity to otherwise simple code.