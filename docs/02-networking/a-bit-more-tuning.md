# GOMAXPROCS, epoll/kqueue, and Scheduler-Level Tuning

Go applications operating at high concurrency levels frequently encounter performance ceilings that are not attributable to CPU saturation. These limitations often stem from runtime-level mechanics: how goroutines (G) are scheduled onto logical processors (P) via operating system threads (M), how blocking operations affect thread availability, and how the runtime interacts with kernel facilities like `epoll` or `kqueue` for I/O readiness.

Unlike surface-level code optimization, resolving these issues requires awareness of the Go scheduler’s internal design, particularly how GOMAXPROCS governs execution parallelism and how thread contention, cache locality, and syscall latency emerge under load. Misconfigured runtime settings can lead to excessive context switching, stalled P’s, and degraded throughput despite available cores.

System-level tuning—through CPU affinity, thread pinning, and scheduler introspection—provides a critical path to improving latency and throughput in multicore environments. When paired with precise benchmarking and observability, these adjustments allow Go services to scale more predictably and fully take advantage of modern hardware architectures.

## Understanding GOMAXPROCS

In Go, `GOMAXPROCS` defines the maximum number of operating system threads (M’s) simultaneously executing user‑level Go code (G’s). It’s set to the developer's machine’s logical CPU count by default. Under the hood, the scheduler exposes P’s (processors) equal to `GOMAXPROCS`. Each P hosts a run queue of G’s and binds to a single M to execute Go code.

```go
package main

import (
    "fmt"
    "runtime"
)

func main() {
    // Show current value
    fmt.Printf("GOMAXPROCS = %d\n", runtime.GOMAXPROCS(0))

    // Set to 4 and confirm
    prev := runtime.GOMAXPROCS(4)
    fmt.Printf("Changed from %d to %d\n", prev, runtime.GOMAXPROCS(0))
}
```

When developers increase `GOMAXPROCS`, developers allow more P’s—and therefore more OS threads—to run Go‑routines in parallel. That often boosts performance for CPU‑bound workloads. However, more P’s also incur more context switches, more cache thrashing, and potentially more contention in shared data structures (e.g., the garbage collector’s work queues). It's important to understand that blindly scaling past the sweet spot can actually degrade latency.

## Diving into Go’s Scheduler Internals

Go’s scheduler organizes three core actors: G (goroutine), M (OS thread), and P (logical processor), [see more details here](networking-internals.md/#goroutines-and-the-runtime-scheduler). When a goroutine makes a blocking syscall, its M detaches from its P, returning the P to the global scheduler so another M can pick it up. This design prevents syscalls from starving CPU‑bound goroutines.

The scheduler uses work stealing: each P maintains a local run queue, and idle P’s will steal work from busier peers. If developers set GOMAXPROCS too high, developers will see diminishing returns in stolen work versus the overhead of balancing those run queues.

Enabling scheduler tracing via `GODEBUG` can reveal fine grained metrics:

```sh
GODEBUG=schedtrace=1000,scheddetail=1 go run main.go
```

- `schedtrace=1000` instructs the runtime to print scheduler state every 1000 milliseconds (1 second).
- `scheddetail=1` enables additional information per logical processor (P), such as individual run queue lengths.

Each printed trace includes statistics like:

```log
SCHED 3024ms: gomaxprocs=14 idleprocs=14 threads=26 spinningthreads=0 needspinning=0 idlethreads=20 runqueue=0 gcwaiting=false nmidlelocked=1 stopwait=0 sysmonwait=false
  P0: status=0 schedtick=173 syscalltick=3411 m=nil runqsize=0 gfreecnt=6 timerslen=0
  ...
  P13: status=0 schedtick=96 syscalltick=310 m=nil runqsize=0 gfreecnt=2 timerslen=0
  M25: p=nil curg=nil mallocing=0 throwing=0 preemptoff= locks=0 dying=0 spinning=false blocked=true lockedg=nil
  ...
```

The first line reports global scheduler state including whether garbage collection is blocking (gcwaiting), if spinning threads are needed, and idle thread counts.

Each P line details the logical processor's scheduler activity, including the number of times it's scheduled (schedtick), system call activity (syscalltick), timers, and free goroutine slots.

The M lines represent OS threads, showing which goroutine (if any) is running, memory allocation status, lock states, and whether the thread is idle, spinning, or blocked.

This structure helps pinpoint not just concurrency bottlenecks but also scheduler inefficiencies, syscall stalls, or underutilized cores. The output reflects:

- `gomaxprocs=14`: Number of logical processors (P’s).
- `idleprocs=14`: All processors are idle, indicating no runnable goroutines.
- `threads=26`: Number of M’s (OS threads) created.
- `spinningthreads=0`: No threads are actively searching for work.
- `needspinning=0`: No additional spinning threads are requested by the scheduler.
- `idlethreads=20`: Number of OS threads currently idle.
- `runqueue=0`: Global run queue is empty.
- `gcwaiting=false`: Garbage collector is not blocking execution.
- `nmidlelocked=1`: One P is locked to a thread that is currently idle.
- `stopwait=0`: No goroutines waiting to stop the world.
- `sysmonwait=false`: The system monitor is actively running, not sleeping.

The global run queue holds goroutines that are not bound to any specific P or that overflowed local queues. In contrast, each logical processor (P) maintains a local run queue of goroutines it is responsible for scheduling. Goroutines are preferentially enqueued locally for performance: local queues avoid lock contention and improve cache locality. It may be placed on the global queue only when a P's local queue is full, or a goroutine originates from outside a P (e.g., from a syscall).

This dual-queue strategy reduces synchronization overhead across P’s and enables efficient scheduling under high concurrency. Understanding the ratio of local vs global queue activity helps diagnose whether the system is under-provisioned, improperly balanced, or suffering from excessive cross-P migrations.

These insights help quantify how efficiently goroutines are scheduled, how much parallelism is actually utilized, and whether the system is under- or over-provisioned in terms of logical processors. Observing these patterns under load is crucial when adjusting `GOMAXPROCS`, diagnosing tail latency, or identifying scheduler contention.

## Netpoller: Deep Dive into epoll on Linux and kqueue on BSD

In any Go application handling high connection volumes, the network poller plays a critical behind-the-scenes role. At its core, Go uses the OS-level multiplexing facilities—`epoll` on Linux and `kqueue` on BSD/macOS—to monitor thousands of sockets concurrently with minimal threads. The runtime leverages these mechanisms efficiently, but understanding how and why reveals opportunities for tuning, especially under demanding loads.

When a goroutine initiates a network operation like reading from a TCP connection, the runtime doesn't immediately block the underlying thread. Instead, it registers the file descriptor with the poller—using `epoll_ctl` in edge-triggered mode or `EV_SET` with `EVFILT_READ`—and parks the goroutine. The actual thread (M) becomes free to run other goroutines. When data arrives, the kernel signals the poller thread, which in turn wakes the appropriate goroutine by scheduling it onto a P’s run queue. This wakeup process minimizes contention by relying on per-P notification lists and avoids runtime lock bottlenecks.

Go uses edge-triggered notifications, which signal only on state transitions—like new data becoming available. This design requires the application to drain sockets fully during each wakeup or risk missing future events. While more complex than level-triggered behavior, edge-triggered mode significantly reduces syscall overhead under load.

Here's a simplified version of what happens under the hood during a read operation:

```go
func pollAndRead(conn net.Conn) ([]byte, error) {
    buf := make([]byte, 4096)
    for {
        n, err := conn.Read(buf)
        if n > 0 {
            return buf[:n], nil
        }
        if err != nil && !isTemporary(err) {
            return nil, err
        }
        // Data not ready yet — goroutine will be parked until poller wakes it
    }
}
```

Internally, Go runs a dedicated poller thread that loops on `epoll_wait` or `kevent`, collecting batches of events (typically 512 at a time). After the call returns, the runtime processes these events, distributing wakeups across logical processors to prevent any single P from becoming a bottleneck. To further promote scheduling fairness, the poller thread may rotate across P’s periodically, a behavior governed by `GODEBUG=netpollWaitLatency`.

Go’s runtime is optimized to reduce unnecessary syscalls and context switches. All file descriptors are set to non-blocking, which allows the poller thread to remain responsive. To avoid the thundering herd problem—where multiple threads wake on the same socket—the poller ensures only one goroutine handles a given FD event at a time.

The design goes even further by aligning the circular event buffer with cache lines and distributing wakeups via per-P lists. These details matter at scale. With proper alignment and locality, Go reduces CPU cache contention when thousands of connections are active.

For developers looking to inspect poller behavior, enabling tracing with `GODEBUG=netpoll=1` can surface system-level latencies and epoll activity. Additionally, the `GODEBUG=netpollWaitLatency=200` flag configures the poller’s willingness to hand off to another P every 200 microseconds. That’s particularly helpful in debugging idle P starvation or evaluating fairness in high-throughput systems.

Here's a small experiment that logs event activity:

```bash
GODEBUG=netpoll=1 go run main.go
```

You’ll see log lines like:

```
runtime: netpoll: poll returned n=3
runtime: netpoll: waking g=102 for fd=5
```

Most developers never need to think about this machinery—and they shouldn't. But these details become valuable in edge cases, like high-throughput HTTP proxies or latency-sensitive services dealing with hundreds of thousands of concurrent sockets. Tuning parameters like `GOMAXPROCS`, adjusting the event buffer size, or modifying poller wake-up intervals can yield measurable performance improvements, particularly in tail latencies.

For example, in a system handling hundreds of thousands of concurrent HTTP/2 streams, increasing `GOMAXPROCS` while using `GODEBUG=netpollWaitLatency=100` helped reduce the 99th percentile read latency by over 15%, simply by preventing poller starvation under I/O backpressure.

As with all low-level tuning, it's not about changing knobs blindly. It's about knowing what Go’s netpoller is doing, why it’s structured the way it is, and where its boundaries can be nudged for just a bit more efficiency—when measurements tell you it’s worth it.

## Thread Pinning with `LockOSThread` and `GODEBUG` Flags

While Go provides mechanisms like `runtime.LockOSThread()` for explicitly pinning goroutines to OS threads, practical benchmarks reveal this rarely provides significant benefits and often introduces complexity without corresponding improvements. Pinning can reduce thread migration and scheduling jitter in extremely latency-sensitive scenarios or real-time contexts. However, measurements consistently show that Go's scheduler efficiently manages workloads without explicit thread pinning, especially for CPU-bound tasks and typical server applications.

If considering thread pinning, thorough benchmarking is mandatory. For instance, using `GODEBUG` flags (`schedtrace`, `scheddetail`) allows deep introspection of scheduler behavior, revealing situations where pinning might alleviate subtle scheduler contention or latency spikes. Nonetheless, benchmarks indicate such cases are uncommon. The overhead and rigidity introduced by thread pinning usually outweigh benefits, except in highly specialized real-time scenarios on isolated hardware with precise scheduling guarantees.

Here's how developers might pin threads cautiously:

```go
runtime.LockOSThread()
defer runtime.UnlockOSThread()

// perform critical latency-sensitive work here
```

Always pair such modifications with extensive metrics collection and scheduler tracing (`GODEBUG=schedtrace=1000,scheddetail=1`) to validate tangible gains over Go’s robust default scheduling behavior.

## CPU Affinity and External Tools

Using external tools like `taskset` or system calls such as `sched_setaffinity` can bind threads or processes to specific CPU cores. While theoretically beneficial for cache locality and predictable performance, extensive benchmarking consistently demonstrates limited practical value in most Go applications.

Explicit CPU affinity management typically helps only in tightly controlled environments with:

- Real-time latency constraints (microsecond-level jitter).
- Dedicated and isolated CPUs (e.g., via Linux kernel’s isolcpus).
- Avoidance of thread migration on NUMA hardware.

Example of cautious CPU affinity usage:

```go
func setAffinity(cpuList []int) error {
    pid := os.Getpid()
    var mask unix.CPUSet
    for _, cpu := range cpuList {
        mask.Set(cpu)
    }
    return unix.SchedSetaffinity(pid, &mask)
}

func main() {
    runtime.LockOSThread()
    defer runtime.UnlockOSThread()

    if err := setAffinity([]int{2, 3}); err != nil {
        log.Fatalf("CPU affinity failed: %v", err)
    }

    // perform critical work with confirmed benefit
}
```

Without dedicated benchmarking and validation, these techniques may degrade performance, starve other processes, or introduce subtle latency regressions. Treat thread pinning and CPU affinity as highly specialized tools—effective only after meticulous measurement confirms their benefit.

---

Tuning Go at the scheduler level can unlock significant performance gains, but it demands an intimate understanding of P’s, M’s, and G’s. Blindly upping `GOMAXPROCS` or pinning threads without measurement can backfire. the advice is to treat these knobs as surgical tools: use `GODEBUG` traces to diagnose, isolate subsystems where affinity or pinning makes sense, and always validate with benchmarks and profiles.

Go’s runtime is ever‑evolving. Upcoming work in preemptive scheduling and user‑level interrupts promises to reduce tail latency further and improve fairness. Until then, these low‑level levers remain some of the most powerful ways to squeeze every drop of performance from developer's Go services.
