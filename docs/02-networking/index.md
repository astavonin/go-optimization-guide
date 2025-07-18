# Practical Networking Patterns in Go

A 15-part guide to building scalable, efficient, and resilient networked applications in Go—grounded in real-world benchmarks, low-level optimizations, and practical design patterns.

---

## Benchmarking First

- [Benchmarking and Load Testing for Networked Go Apps](bench-and-load.md)

	Establish performance baselines before optimizing anything. Learn how to simulate realistic traffic using tools like `vegeta`, `wrk`, and `k6`. Covers throughput, latency percentiles, connection concurrency, and profiling under load. Sets the foundation for diagnosing bottlenecks and measuring the impact of every optimization in the series.

---

## Foundations and Core Concepts

- [How Go Handles Networking: Concurrency, Goroutines, and the net Package](networking-internals.md)

	Understand Go’s approach to networking from the ground up. Covers how goroutines, the `net` package, and the runtime scheduler interact, including blocking I/O behavior, connection handling, and the use of pollers like `epoll` or `kqueue` under the hood.

- [Efficient Use of `net/http`, `net.Conn`, and UDP](efficient-net-use.md)

	Compare idiomatic and advanced usage of `net/http` vs raw `net.Conn`. Dive into connection pooling, custom dialers, stream reuse, and buffer tuning. Demonstrates how to avoid common pitfalls like leaking connections, blocking handlers, or over-allocating buffers.

---

## Scaling and Performance Engineering

- [Managing 10K++ Concurrent Connections in Go](10k-connections.md)

	Handling massive concurrency requires intentional architecture. Explore how to efficiently serve 10,000+ concurrent sockets using Go’s goroutines, proper resource capping, socket tuning, and runtime configuration. Focuses on connection lifecycles, scaling pitfalls, and real-world tuning.

- [GOMAXPROCS, epoll/kqueue, and Scheduler-Level Tuning](a-bit-more-tuning.md)

	Dive into low-level performance knobs like `GOMAXPROCS`, `GODEBUG`, thread pinning, and how Go’s scheduler interacts with epoll/kqueue. Learn when increasing parallelism helps—and when it doesn’t. Includes tools for CPU affinity and benchmarking the effect of these changes.

---

## Diagnostics and Resilience

- [Building Resilient Connection Handling with Load Shedding and Backpressure](resilient-connection-handling.md)

	Learn how to prevent overloads from crashing your system. Covers circuit breakers, passive vs active load shedding, backpressure strategies using channel buffering and timeouts, and how to reject or degrade requests gracefully under pressure.

- [Memory Management and Leak Prevention in Long-Lived Connections](long-lived-connections.md)

	Long-lived connections like WebSockets or TCP streams can slowly leak memory or accumulate goroutines. This post shows how to identify common leaks, enforce read/write deadlines, manage backpressure, and trace heap growth with memory profiles.

---

## Transport-Level Optimization

- [Comparing TCP, HTTP/2, and gRPC Performance in Go](tcp-http2-grpc.md)

	Benchmark and analyze different transport protocols in Go: raw TCP with custom framing, HTTP/2 via `net/http`, and gRPC. Evaluate latency, throughput, connection reuse, and CPU/memory cost across real scenarios like internal APIs, messaging systems, and microservices.

- [QUIC in Go: Building Low-Latency Services with quic-go](quic-in-go.md)

	Explore QUIC as a next-gen transport for real-time and mobile-first systems. Introduce the `quic-go` library, demonstrate setup for secure multiplexed streams, and compare performance against HTTP/2 and TCP. Also covers connection migration and 0-RTT for fast startup.

---

## Low-Level and Advanced Tuning

- [Low-Level Network Optimizations: Socket Options That Matter](low-level-optimizations.md)

	Explore advanced socket-level tuning options like disabling Nagle’s algorithm (`TCP_NODELAY`), adjusting `SO_REUSEPORT`, `SO_RCVBUF`/`SO_SNDBUF`, TCP keepalives, and connection backlog (`SOMAXCONN`). Explain how Go exposes these via `syscall` and how to wrap them safely. Real-world examples included for latency-sensitive systems and high-throughput services.

- [Tuning DNS Performance in Go Services](dns_performance.md)

	DNS lookups are often overlooked as latency culprits. Learn how Go performs name resolution (cgo vs Go resolver), when to cache results, and how to use custom dialers or pre-resolved IPs to avoid flaky network paths. Includes metrics and debugging tips for real-world DNS slowdowns.

- [Optimizing TLS for Speed: Handshake, Reuse, and Cipher Choice](tls-for-speed.md)

	TLS adds security—but it can also add overhead. Tune your Go service for fast and secure TLS: enable session resumption, choose fast cipher suites, use ALPN negotiation wisely, and minimize cert verification cost. Examples included with `tls.Config` best practices.

- [Connection Lifecycle Observability: From Dial to Close](connection_observability.md)

	Trace the full lifecycle of a connection with visibility at each stage—DNS, dial, handshake, negotiation, reads/writes, and teardown. Learn how to log connection spans, trace hangs, correlate errors with performance metrics, and build custom observability into network flows.