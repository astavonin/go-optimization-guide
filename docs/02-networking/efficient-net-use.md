# Efficient Use of `net/http`, `net.Conn`, and UDP in High-Traffic Go Services

When we first start building high-traffic services in Go, we often lean heavily on `net/http`. It’s stable, ergonomic, and remarkably capable for 80% of use cases. But as soon as traffic spikes or latency budgets shrink, the cracks begin to show.

It’s not that `net/http` is broken—it’s just that the defaults are tuned for convenience, not for performance under stress. And as we scale backend services to handle millions of requests per second, understanding what happens underneath the abstraction becomes the difference between meeting SLOs and fire-fighting in production.

This article is a walkthrough of how to make networked Go services truly efficient—what works, what breaks, and how to go beyond idiomatic usage. We’ll start with `net/http`, drop into raw `net.Conn`, and finish with real-world patterns for handling UDP in latency-sensitive systems.

## The Hidden Complexity Behind a Simple HTTP Call

Let’s begin where most Go developers do: a simple `http.Client`.

```go
client := &http.Client{
    Timeout: 5 * time.Second,
}

resp, err := client.Get("http://localhost:8080/data")
if err != nil {
    log.Fatal(err)
}
defer resp.Body.Close()
```

This looks harmless. It gets the job done, and in most local tests, it performs reasonably well. But in production, at scale, this innocent-looking code can trigger a surprising range of issues: leaked connections, memory spikes, blocked goroutines, and mysterious latency cliffs.

One of the most common issues is forgetting to fully read `resp.Body` before closing it. [Go’s HTTP client won’t reuse connections unless the body is drained](https://github.com/google/go-github/pull/317). And under load, that means you're constantly opening new TCP connections—slamming the kernel with ephemeral ports, exhausting file descriptors, and triggering throttling.

Here’s the safe pattern:

```go
io.Copy(io.Discard, resp.Body)
defer resp.Body.Close()
```

## Transport Tuning: When Defaults Aren’t Enough

It’s easy to overlook how much global state hides behind `http.DefaultTransport`. If you spin up multiple `http.Client` instances across your app without customizing the transport, you're probably reusing a shared global pool without realizing it.

This leads to unpredictable behavior under load: idle connections get evicted too quickly, or keep-alive connections linger longer than they should. The fix? Build a tuned `Transport` that matches your concurrency profile.

### Custom `http.Transport` Fields to Tune

All the following settings are part of the `http.Transport` struct:

```go
transport := &http.Transport{
    MaxIdleConns:          1000,
    MaxConnsPerHost:       100,
    IdleConnTimeout:       90 * time.Second,
    ExpectContinueTimeout: 0,
    DialContext: (&net.Dialer{
        Timeout:   5 * time.Second,
        KeepAlive: 30 * time.Second,
    }).DialContext,
}

client := &http.Client{
    Transport: transport,
    Timeout:   2 * time.Second,
}
```

## More Advanced Optimization Tricks

These are all tied to key settings in the `http.Transport`, `http.Client`, and `http.Server` structs, or custom wrappers built on top of them:

### Set `ExpectContinueTimeout` Carefully

If our clients send large POST requests and the server doesn’t support `100-continue` properly, we can reduce or eliminate this delay:

```go
transport := &http.Transport{
    ...
    ExpectContinueTimeout: 0, // if not needed, skip the wait entirely
    ...
}
```

### Constrain `MaxConnsPerHost`

Go’s default HTTP client will open an unbounded number of connections to a host. That’s fine until one of your downstreams can’t handle it.

```go
transport := &http.Transport{
    ...
    MaxConnsPerHost: 100,
    ...
}
```

This prevents stampedes during spikes and avoids exhausting resources on your backend services.

### Use Small `http.Client.Timeout`

A common mistake is setting a very high timeout (e.g., 30s) for safety. But long timeouts hold onto goroutines, buffers, and sockets under pressure. Prefer tighter control:

```go
client := &http.Client{
    Timeout: 2 * time.Second,
}
```

Instead of relying on big timeouts, use retries with backoff (e.g., with go-retryablehttp) to improve resiliency under partial failure.

### Explicitly Set `ReadBufferSize` and `WriteBufferSize` in `http.Server`

Go's `http.Server` does not expose `ReadBufferSize` and `WriteBufferSize` directly, but when you need to reduce GC pressure and improve syscall efficiency under load, you can pre-size the buffers in custom `Conn` wrappers. 4KB–8KB is a balanced value for most workloads: it's large enough to handle small headers and bodies efficiently without wasting memory. For example, 4KB covers almost all typical HTTP headers and small JSON payloads.

You can implement this using `bufio.NewReaderSize` and `NewWriterSize` in a wrapped connection that plugs into a custom `net.Listener` and `http.Server.ConnContext`.

If you're using `fasthttp`, you can configure buffer sizes explicitly:

```go
server := &fasthttp.Server{
    ReadBufferSize:  4096,
    WriteBufferSize: 4096,
    ...
}
```

This avoids dynamic allocations on each request and leads to more predictable memory usage and cache locality under high throughput.

### Use `bufio.Reader.Peek()` for Efficient Framing

When you're building a framed protocol over TCP (e.g., length-prefixed binary messages), calling `Read()` repeatedly can introduce syscall overhead and poor performance due to fragmented reads. Using `Peek()` allows you to look ahead into the buffered data without consuming it, enabling efficient protocol detection and boundary handling. This is especially useful in multiplexed systems or streaming services where precise message boundaries matter.

```go
header, _ := reader.Peek(8) // Peek without advancing the buffer
```

### Force Fresh DNS Lookups with Custom Dialers

Go caches DNS results for the lifetime of the process. This can lead to stale IPs when using short-lived infrastructure like Kubernetes pods, where service IPs shift frequently. To ensure timely failover and routing, you can create a new `Dialer` per request or periodically reset the client.

Here's how to bypass Go's internal DNS cache:

```go
DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
    return (&net.Dialer{}).DialContext(ctx, network, addr)
},
```

This ensures a fresh DNS lookup per request. While this adds minor overhead, it's necessary in failover-sensitive environments.

### Use `sync.Pool` for Readers/Writers

Most people use `sync.Pool` to reuse `[]byte` buffers, but for services that process many requests per second, allocating `bufio.Reader` and `bufio.Writer` objects per connection adds up. These objects also maintain their own buffers, so recycling them reduces pressure on both heap allocations and garbage collection.

```go
var readerPool = sync.Pool{
    New: func() interface{} {
        return bufio.NewReaderSize(nil, 4096)
    },
}

func getReader(conn net.Conn) *bufio.Reader {
    r := readerPool.Get().(*bufio.Reader)
    r.Reset(conn)
    return r
}
```

This practice significantly reduces allocation churn and improves latency consistency, especially in systems processing thousands of connections concurrently.

### Don’t Share `http.Client` Across Multiple Hosts

While it might seem efficient to reuse a single `http.Client`, each target host maintains its own internal connection pool within the underlying `http.Transport`. If you use the same client for multiple base URLs, you end up mixing connection reuse and causing head-of-line blocking across unrelated services. Worse, DNS caching and socket exhaustion become harder to track.

Instead, define one `http.Client` per logical upstream service, especially if you operate within a service mesh or call many third-party APIs. This approach improves isolation and predictability.

### Use `ConnContext` and `ConnState` Hooks for Debugging

These hooks are invaluable when diagnosing long-lived connections, memory leaks, or resource exhaustion in production. `ConnState` lets you track transitions such as `StateNew`, `StateActive`, `StateIdle`, and `StateHijacked` per connection.

By monitoring these events, you can detect when connections hang, fail to close, or unexpectedly idle out. It also helps when correlating behavior with client IPs or network zones.

```go
ConnState: func(conn net.Conn, state http.ConnState) {
    log.Printf("conn %v changed state to %v", conn.RemoteAddr(), state)
},
```

## Dropping the Abstraction: When to Use `net.Conn`

As we get closer to the limits of what the Go standard library can offer, it’s worth knowing that there are high-performance alternatives built specifically for event-driven, low-latency workloads. Projects like [`cloudwego/netpoll`](https://github.com/cloudwego/netpoll) and [`tidwall/evio`](https://github.com/tidwall/evio) offer powerful tools for maximizing performance beyond what’s achievable with `net.Conn` alone.

- **[`cloudwego/netpoll`](https://github.com/cloudwego/netpoll)** is an epoll-based network library designed for building massive concurrent network services with minimal GC overhead. It uses event-based I/O to eliminate goroutine-per-connection costs, ideal for scenarios like RPC proxies, internal service meshes, or high-frequency messaging systems.

- **[`tidwall/evio`](https://github.com/tidwall/evio)** provides a fast, non-blocking event loop for Go based on the [reactor pattern](https://en.wikipedia.org/wiki/Reactor_pattern). It’s well-suited for protocols where latency matters more than per-connection state complexity, such as custom TCP, UDP protocols, or lightweight gateways.

If you're building systems where throughput or connection count exceeds hundreds of thousands, or where tail latency is critical, it's worth exploring these libraries. They come with trade-offs—most notably, less standardization and more manual lifecycle management—but in return, they give you fine-grained control over performance-critical paths.


Sometimes, even a tuned HTTP stack isn't enough.

In cases like internal binary protocols or services dealing with hundreds of thousands of requests per second, we may find we're paying for HTTP semantics we don't use. Dropping to `net.Conn` gives us full control—no pooling surprises, no hidden keep-alives, just a raw socket.

```go
ln, err := net.Listen("tcp", ":9000")
...
```

This lets us take over the connection lifecycle, buffering, and concurrency fully. It also opens up opportunities to reduce GC impact via buffer reuse:

```go
var bufPool = sync.Pool{
    New: func() interface{} {
        return make([]byte, 4096)
    },
}
```

Enabling TCP\_NODELAY is useful in latency-sensitive systems:

```go
tcpConn := conn.(*net.TCPConn)
tcpConn.SetNoDelay(true)
```

## Beyond TCP: Why UDP Matters

TCP could be too heavy for workloads like log firehose ingestion, telemetry beacons, or heartbeat messages. We can turn to UDP for low-latency, connectionless data delivery:

```go
conn, _ := net.ListenUDP("udp", &net.UDPAddr{Port: 9999})
...
```

This skips handshakes and reuses the socket efficiently. But remember—UDP offers no ordering, reliability, or built-in session tracking. It works best in high-volume, low-consequence pipelines.

## Choosing the Right Tool

Our networking strategy should reflect traffic shape and protocol expectations:

| Scenario                       | Preferred Tool           |
| ------------------------------ | ------------------------ |
| REST/gRPC, general APIs        | `net/http`               |
| HTTP under load                | Tuned `http.Transport`   |
| Custom TCP protocol            | `net.Conn`               |
| Framed binary data             | `net.Conn` + buffer mgmt |
| Fire-and-forget telemetry      | `UDPConn`                |
| Latency-sensitive game updates | `UDP`                    |

---

At scale, network performance is never just about the network. It's also about memory pressure, context lifecycles, kernel behavior, and socket hygiene. We can go far with the Go standard library, but when systems push back, we need to push deeper.

The good news? Go gives us the tools. We just need to use them wisely.

If you're experimenting with framed protocols, zero-copy parsing, or custom benchmarking setups, there's a lot more to explore. Let's keep going.