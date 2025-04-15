# Managing 100K+ Concurrent Connections in Go

Handling massive concurrency in Go is often romanticized—*"goroutines are cheap, just spawn them!"*—but reality gets harsher as we push towards six-digit concurrency levels. Serving 100,000+ concurrent sockets isn't just about throwing hardware at the problem; it's about building architecture that respects the OS, runtime, and network layers.

## Embracing Go’s Concurrency Model

Go’s lightweight goroutines and its powerful runtime scheduler make it an excellent choice for scaling network applications. Goroutines consume only a few kilobytes of stack space, which, in theory, makes them ideal for handling tens of thousands of concurrent connections. However, reality forces us to think beyond just spinning up goroutines. While the language’s abstraction makes concurrency almost “magical,” achieving true efficiency at this scale demands intentional design.

When you run a server that spawns one goroutine per connection, you’re effectively relying on the runtime scheduler to context-switch between thousands of execution threads. It’s crucial to understand that although goroutines are relatively inexpensive, they aren’t free—each one contributes to memory usage and scheduling overhead. Thus, the first design pattern that should be adopted is to ensure that each connection follows a clearly defined lifecycle and that every goroutine performs its task as efficiently as possible.

Let’s consider a basic model where we accept connections and delegate their handling to separate goroutines:

```go
package main

import (
	"log"
	"net"
	"sync/atomic"
	"time"
)

var activeConnections uint64

func main() {
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("Error starting TCP listener: %v", err)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			continue
		}

		atomic.AddUint64(&activeConnections, 1)
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer func() {
		conn.Close()
		atomic.AddUint64(&activeConnections, ^uint64(0)) // effectively decrements the counter
	}()

	// Imagine complex processing here—an echo server example:
	buffer := make([]byte, 1024)
	for {
		conn.SetDeadline(time.Now().Add(30 * time.Second)) // prevent idle hangs
		n, err := conn.Read(buffer)
		if err != nil {
			log.Printf("Connection read error: %v", err)
			return
		}
		_, err = conn.Write(buffer[:n])
		if err != nil {
			log.Printf("Connection write error: %v", err)
			return
		}
	}
}
```

Each accepted connection is processed in its own goroutine in the sample above. Although this model is straightforward and leverages Go’s strengths, it’s only part of the solution. When scaling to 100K+ connections, the design must extend to handle resource limits and prevent runaway resource consumption.

### Architectural Considerations and Resource Capping

Scaling isn’t just about accepting connections—it’s about managing them throughout their entire lifecycle. One common pitfall is allowing the uncontrolled creation of goroutines, which can lead to memory exhaustion or overwhelm the scheduler. Implementing controlled resource capping through a semaphore-like mechanism is essential.

For example, you might limit the number of simultaneous active connections before spinning up a new goroutine for each incoming connection. This strategy might involve a buffered channel acting as a semaphore:

```go
package main

import (
	"net"
)

var connLimiter = make(chan struct{}, 100000) // Max 100K concurrent conns

func main() {
	ln, _ := net.Listen("tcp", ":8080")
	defer ln.Close()

	for {
		conn, _ := ln.Accept()

		connLimiter <- struct{}{} // Acquire slot
		go func(c net.Conn) {
			defer func() {
				c.Close()
				<-connLimiter // Release slot
			}()
			// Dummy echo logic
			buf := make([]byte, 1024)
			c.Read(buf)
			c.Write(buf)
		}(conn)
	}
}
```

This pattern not only helps prevent resource exhaustion but also gracefully degrades service under high load. Adjusting these limits according to your hardware and workload characteristics is a continuous tuning process.

!!! info
	We use the `connLimiter` approach here for purely illustrative purposes, as it clarifies the idea. In real life, you will most likely use [errgroup](https://pkg.go.dev/golang.org/x/sync/errgroup) to manage the goroutines amount and some `SIGINT,` and `SIGTERM` signal handling for graceful process termination.

### OS-Level and Socket Tuning

Before your Go application can even accept 100K+ connections, your operating system must be tuned to handle such loads. This means adjusting parameters like file descriptor limits and TCP stack configurations. For Linux environments, you typically need to increase the maximum number of open file descriptors:

```go
# Increase file descriptor limit
ulimit -n 200000
```

But it doesn’t stop there. You’ll also need:

```bash
sysctl -w net.core.somaxconn=65535
sysctl -w net.ipv4.ip_local_port_range="10000 65535"
sysctl -w net.ipv4.tcp_tw_reuse=1
sysctl -w net.ipv4.tcp_fin_timeout=15
```

- `net.core.somaxconn=65535`: This controls the size of the pending connection queue (the backlog) for listening sockets. A small value here will cause connection drops when many clients attempt to connect simultaneously.
- `net.ipv4.ip_local_port_range="10000 65535"`: Defines the ephemeral port range used for outbound connections. A wider range prevents port exhaustion when you’re making many outbound connections from the same machine.
- `net.ipv4.tcp_tw_reuse=1`: Allows reuse of sockets in `TIME_WAIT` state for new connections if safe. Helps reduce socket exhaustion, especially in short-lived TCP connections.
- `net.ipv4.tcp_fin_timeout=15`: Reduces the time the kernel holds sockets in `FIN_WAIT2` after a connection is closed. Shorter timeout means faster resource reclamation, crucial when thousands of sockets churn per minute.

Tweaking these parameters ensures that OS-level constraints don’t throttle your application. Additionally, socket options such as `TCP_NODELAY` can be instrumental in reducing latency by turning off [Nagle’s algorithm](https://en.wikipedia.org/wiki/Nagle%27s_algorithm). For example, you can set these options using Go’s `net` package wrappers or by leveraging the syscall package for more granular control.

In some cases, using Go’s `net.ListenConfig` allows you to inject custom control over socket creation. This is particularly useful when you need to set options at the time of listener creation:

```go
func main() {
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			var controlErr error
			err := c.Control(func(fd uintptr) {
				// Enable TCP_NODELAY on the socket
				controlErr = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, syscall.TCP_NODELAY, 1)
			})
			if err != nil {
				return err
			}
			return controlErr
		},
	}
	listener, err := lc.Listen(context.Background(), "tcp", ":8080")
	if err != nil {
		log.Fatalf("Error creating listener: %v", err)
	}
	defer listener.Close()
	// Accept connections in a loop…
}
```

### Go Scheduler and Memory Pressure

Running 100K goroutines might sound impressive, but their impact heavily depends on their behavior. If goroutines spend their time blocked on network or disk I/O, Go’s scheduler can efficiently park and resume them with minimal overhead. However, suppose these goroutines frequently allocate memory, spin in busy loops, or engage in tight synchronization via channels or mutexes. In that case, you risk triggering significant GC activity and scheduler churn—ultimately degrading performance.

Garbage collection can become a significant bottleneck in applications with heavy allocation patterns. Go’s GC is optimized for low latency, but when hundreds of thousands of goroutines frequently allocate objects, GC cycles intensify, causing latency spikes and reducing throughput.

To manage this, you can explicitly tune the GC aggressiveness:

```bash
GOGC=50
```

Or directly within your codebase:

```go
import "runtime/debug"

func main() {
    debug.SetGCPercent(50)
    // rest of your application logic
}
```

The default value for `GOGC` is 100, meaning the GC triggers when the heap size doubles compared to the previous GC cycle. Lower values (like 50) mean more frequent but shorter GC cycles, helping control memory growth at the cost of increased CPU overhead.

!!! info
	In some cases, you may need an opposite – [to increase the `GOGC` value, turn the GC off completely](/01-common-patterns/gc/#gc-tuning-gogc), or prefer [GOMEMLIMIT=X and GOGC=off](/01-common-patterns/gc/#gomemlimitx-and-gogcoff-configuration) configuration. **Do not make a decision before careful profiling!**

### Optimizing Goroutine Behavior

Consider structuring your application so that goroutines block naturally rather than actively waiting or spinning. For example, instead of polling channels in tight loops, use select statements efficiently:

```go
for {
    select {
    case msg := <-msgChan:
        handleMsg(msg)
    case <-ctx.Done():
        return
    }
}
```

If your goroutines must wait, prefer blocking on channels or synchronization primitives provided by Go, like mutexes or condition variables, instead of actively polling.

### Pooling and Reusing Objects

Another crucial technique to reduce memory allocations and GC overhead [is using `sync.Pool`](/01-common-patterns/object-pooling):

```go
var bufPool = sync.Pool{
    New: func() any { return make([]byte, 1024) },
}

func handleRequest() {
    buf := bufPool.Get().([]byte)
    defer bufPool.Put(buf)	// (1)

    // use buffer for request handling
}
```

1. Be careful here! It depends on your workflow when you must return an object to the pool!

Reusing objects through pools significantly reduces allocation frequency, thereby lowering the GC overhead and improving the application's responsiveness.

### Connection Lifecycle Management

Every connection that hits the server undergoes a lifecycle—from initialization through data exchange to graceful termination. One common challenge is managing idle connections and ensuring they don’t become inadvertent resource hogs. Setting read and write deadlines, as illustrated below, is crucial. Furthermore, implementing an application-level heartbeat or ping mechanism can aid in detecting stale connections and initiating appropriate cleanup routines.

In one case, a slight delay in the client’s response caused goroutines to accumulate because they were blocked in a read operation. By incorporating deadlines and periodic health checks, the number of zombie goroutines was reduced, significantly improving both stability and resource usage.

At the core of this strategy is a lightweight connection handler. Each incoming TCP connection is handled in its own goroutine, allowing the server to scale with thousands of concurrent clients:

```go
for {
	conn, err := ln.Accept()
	if err != nil {
		// handle error
	}
	go handle(conn)
}
```

Inside the handler, a ticker is used to fire every few seconds, triggering a periodic heartbeat that keeps the connection active and responsive:

```go
ticker := time.NewTicker(5 * time.Second)
defer ticker.Stop()
```

Before reading from the client, the server sets a read deadline—if no data is received within that time, the operation fails, and the connection is cleaned up. This prevents a blocked read from stalling the goroutine indefinitely:

```go
conn.SetReadDeadline(time.Now().Add(10 * time.Second))
_, err := reader.ReadString('
')
if err != nil {
	return // read timeout or client gone
}
```

Likewise, before sending the heartbeat, the server sets a write deadline. If the client is unresponsive or the network is slow, the write will fail promptly, avoiding resource leakage:

```go
select {
case <-ticker.C:
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	conn.Write([]byte("ping"))
default:
	// skip heartbeat if not due
}
```

The result is a loop that reads incoming messages while periodically sending heartbeats, bounded by deadlines on both ends. This simple structure ensures that every connection is actively monitored and that silent failures are detected early—striking a balance between performance and safety.

## Real-World Tuning and Scaling Pitfalls

Scaling to 100K+ connections is not just a matter of code—it’s about anticipating and mitigating potential pitfalls:

- Memory Footprint: While minimal in isolation, each goroutine contributes to the overall memory footprint. Monitoring memory consumption and being vigilant about leaks is essential.
- File Descriptor Limits: Even with Go’s efficiency, the underlying operating system has finite limits. Properly tuning these limits prevents unexpected application crashes.
- Blocking I/O: Improperly managed I/O operations can lead to goroutines blocking indefinitely, causing backlogs and increased latency. Using deadlines and asynchronous patterns where appropriate can mitigate this risk.
- Profiling and Observability: Without proactive profiling, issues like high CPU utilization, goroutine leaks, or unexpected latency spikes can go unnoticed until they escalate into major problems.

You will spend considerable time refining these aspects in production environments, often cycling through iterations of profiling, code optimization, and infrastructure adjustments. Each iteration deepened your understanding of both Go’s runtime behavior and the intricacies of modern network operating systems.