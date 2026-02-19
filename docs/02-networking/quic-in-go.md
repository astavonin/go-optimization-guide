# QUIC in Go: Building Low-Latency Services with quic-go

QUIC has emerged as a robust protocol, solving many inherent limitations of traditional TCP connections. QUIC combines encryption, multiplexing, and connection migration into a unified protocol, designed to optimize web performance, particularly in real-time and mobile-first applications. In Go, [quic-go](https://github.com/quic-go/quic-go) is the main QUIC implementation and serves as a practical base for building efficient, low-latency network services with built-in encryption and stream multiplexing.

## Understanding QUIC

Originally developed at Google and later standardized by the IETF, QUIC rethinks the transport layer to overcome longstanding TCP limitations:

* **Head-of-line blocking:** TCP delivers a single ordered byte stream, so packet loss stalls everything behind it. QUIC splits data into independent streams, allowing others to proceed even when one is delayed.
* **Per-packet encryption and header protection:** QUIC applies encryption at the packet level, including selective header protection tied to packet numbers—something DTLS’s record-based framing can’t support.
* **Built-in transport mechanisms:** QUIC handles stream multiplexing, flow control, and retransmissions as part of the protocol. DTLS, by contrast, only secures datagrams and leaves reliability and ordering to the application.
* **Connection ID abstraction:** QUIC identifies sessions using connection IDs rather than IP and port tuples, allowing connections to persist across network changes. DTLS provides no such abstraction, making mobility difficult to implement.

### QUIC vs. TCP: Key Differences

QUIC takes a fundamentally different approach from TCP. While TCP is built directly on IP and requires a connection-oriented handshake before data can flow, QUIC runs over UDP and handles its own connection logic, reducing setup overhead and improving startup latency. This architectural choice allows QUIC to provide multiplexed, independent streams that effectively eliminate the head-of-line blocking issue commonly experienced with TCP, where the delay or loss of one packet stalls subsequent packets.

QUIC integrates TLS 1.3 directly into its transport layer, eliminating the layered negotiation seen in TCP+TLS. This design streamlines the handshake process and enables 0-RTT data, where repeat connections can begin transmitting encrypted payloads immediately—something TCP simply doesn’t support.

Another key distinction is how connections are identified. TCP connections are bound to a specific IP and port, so any change in network interface results in a broken connection. QUIC avoids this by using connection IDs that remain stable across address changes, allowing sessions to continue uninterrupted when a device moves between networks—critical for mobile and latency-sensitive use cases.

## Is QUIC Based on DTLS?

Although QUIC and DTLS both use TLS cryptographic primitives over UDP, QUIC does *not* build on DTLS. Instead, QUIC incorporates **TLS 1.3 directly into its transport layer**, inheriting only the cryptographic handshake—not the record framing or protocol structure of DTLS.

QUIC defines its own packet encoding, multiplexing, retransmission, and encryption formats. It wraps TLS handshake messages within QUIC packets and tightly couples encryption state with transport features like packet numbers and stream IDs. In contrast, DTLS operates as a secured datagram layer atop UDP, providing encryption and authentication but leaving transport semantics—such as retransmit, ordering, or flow control—to higher layers.

The reasons for QUIC rejecting DTLS as its security base include:

* **Tighter integration of handshake and transport**: QUIC merges TLS negotiation with transport state setup, enabling 0‑RTT reuse and 1‑RTT setup in fewer round trips. DTLS’s layered model introduces higher latency.
* **Fine-grained encryption control**: QUIC encrypts packet headers and payloads per-packet, bound to packet number and header offset. This is impossible with DTLS’s coarse record layer.
* **Native transport features**: QUIC implements multiplexed, independent streams, per-stream flow control, and resilient retransmission logic. DTLS treats reliability and ordering as the application's responsibility.
* **Connection migration capability**: QUIC uses connection IDs decoupled from IP/port endpoints, enabling smooth network-interface switches. DTLS lacks this architectural property.

In summary, QUIC uses TLS 1.3 for cryptographic handshake but **eschews DTLS entirely**, replacing it with a tightly integrated transport protocol. This design empowers QUIC to offer secure, low-latency, multiplexed, and mobile-friendly connections that DTLS—optimized for secure datagram channels—cannot match.

## Introducing quic-go

quic-go implements the core IETF QUIC specification and supports most features required for production use, including TLS 1.3 integration, 0-RTT, stream multiplexing, and flow control. While some advanced capabilities like active connection migration are not yet implemented.

### Getting Started with quic-go

To start using `quic-go`, include it via Go modules:

```bash
go get github.com/quic-go/quic-go
```

### Basic QUIC Server

A basic QUIC server setup in Go is conceptually similar to writing a traditional TCP server using the `net` package, but with several important distinctions.

The initialization phase still involves listening on an address, but uses `quic.ListenAddr()` instead of `net.Listen()`. Unlike TCP, QUIC operates over UDP and requires a TLS configuration from the start, as all QUIC connections are encrypted by design. There’s no need to manually wrap connections in TLS—QUIC handles encryption as part of the protocol.

```go
{%
    include-markdown "02-networking/src/quic_server.go"
    start="// quic-server-init-start"
    end="// quic-server-init-end"
%}
```

After accepting a connection, handling diverges more significantly from the traditional `net.Conn` model. A single QUIC connection supports multiple independent streams, each functioning like a lightweight, ordered, bidirectional byte stream. These are accepted and handled independently, allowing concurrent interactions over a single connection without head-of-line blocking.

```go
{%
    include-markdown "02-networking/src/quic_server.go"
    start="// quic-server-handle-start"
    end="// quic-server-handle-end"
%}
```

This separation of initialization and per-stream handling is one of QUIC's most powerful features. With TCP, one connection equals one stream. With QUIC, one connection can carry dozens of concurrent, fully independent streams with isolated flow control and recovery behavior, allowing high-efficiency communication patterns with minimal latency.

## Multiplexed Streams

QUIC inherently supports stream multiplexing, enabling simultaneous bidirectional communication without additional connection overhead. Streams operate independently, preventing head-of-line blocking, thus enhancing throughput and reducing latency.

```go
stream, err := session.OpenStreamSync(context.Background())
if err != nil {
	log.Fatal(err)
}
_, err = stream.Write([]byte("Hello QUIC!"))
```

## Performance: QUIC vs. HTTP/2 and TCP

In performance benchmarks, QUIC frequently outperforms traditional HTTP/2 over TCP, particularly on lossy networks common in mobile environments. QUIC recovers faster from packet loss due to multiplexed streams and built-in congestion control algorithms like Cubic and BBR, integrated directly into the quic-go library.

## Connection Migration

One significant advantage of the QUIC protocol is its support for seamless connection migration(1), designed to allow mobile devices to maintain connections while switching networks (e.g., from Wi-Fi to cellular). This is enabled by connection IDs, which abstract away the client's IP address and port, allowing the server to continue communication even if the client's network path changes.
{ .annotate }

1. See [9.2. Initiating Connection Migration](https://datatracker.ietf.org/doc/html/rfc9000?section-9.2#name-initiating-connection-migra) and [9.3. Responding to Connection Migration](https://datatracker.ietf.org/doc/html/rfc9000?section-9.2#name-responding-to-connection-mi) sections from [RFC 9000](https://datatracker.ietf.org/doc/html/rfc9000).

However, in practice, connection migration depends on the specific implementation. In quic-go (as of v0.52.0), full active migration is not yet implemented. Here's what is supported:

* **NAT rebinding works:** If a client's IP or port changes due to NAT behavior or DHCP renewal, and the same connection ID is used, quic-go will continue the session without requiring a new connection. This is passive migration and requires no explicit action from the client.
* **Interface switching (active migration) is not supported:** Switching network interfaces—such as moving from Wi-Fi to LTE—requires sending packets from a new path and validating it with PATH_CHALLENGE and PATH_RESPONSE frames. The protocol defines this behavior, but quic-go does not implement it.


## Resilience in Practice

QUIC is particularly effective in networks where reliability is difficult to guarantee—mobile clients on LTE, IoT nodes on lossy links, or any edge system moving between access points. Rather than relying on techniques like forward error correction (dropped during IETF standardization), QUIC builds resilience into the core transport through a combination of stream isolation, fine-grained recovery, and flexible routing.

QUIC connections are multiplexed: each stream runs independently with a separate flow control and delivery state. Packet loss on one stream doesn’t interfere with others, avoiding the head-of-line blocking inherent in TCP. This alone gives QUIC a noticeable advantage in throughput on degraded links.

Loss recovery in QUIC is driven by packet-level acknowledgments and RTT-based timers—no reliance on TCP-style retransmission logic. Lost packets are detected faster, and retransmissions are scoped to the specific frames involved.

```mermaid
sequenceDiagram
    participant Client
    participant Server

    Note over Client, Server: RTT = 100ms

    Client->>Server: Packet #1 (STREAM)
    Client->>Server: Packet #2 (STREAM)
    Client--xServer: Packet #3 (lost)
    Client->>Server: Packet #4 (STREAM)

    Note over Server: Server receives #1, #2, #4 (but not #3)

    Server-->>Client: ACK [1-2, 4] after 100ms

    Note over Client: Missing #3 is inferred lost\nonce #4 is ACKed before it

    Client->>Server: Packet #5 (Retransmit lost frame from #3)
    Server-->>Client: ACK [5]
```

### How QUIC ACKs Differ from TCP

While TCP includes support for SACK (Selective Acknowledgment), there are critical distinctions that make QUIC’s loss recovery more robust and deterministic:

* **SACK in TCP is optional** and negotiated during handshake. QUIC’s ACK ranges are always enabled and part of the core protocol.
* **TCP acknowledges bytes**, while QUIC acknowledges whole packets by number.
* **Retransmissions in TCP** are often byte-specific and tied to stream position. In QUIC, entire packets are retransmitted.
* **QUIC ACKs are encrypted**, making them tamper-resistant and resilient to on-path interference.

| Feature                    | TCP                    | QUIC                    |
| -------------------------- | ---------------------- | ----------------------- |
| ACK granularity            | Byte-level             | Packet-level            |
| ACK range support          | Optional (via SACK)    | Mandatory               |
| Loss detection             | Duplicate ACKs, timers | Gaps + RTT-based timers |
| Retransmission granularity | Partial stream bytes   | Full packet             |
| Encryption of ACKs         | No                     | Yes                     |

### Connection Migration and Middlebox Resilience

When a device moves between networks (e.g., Wi-Fi to LTE), QUIC's use of connection IDs allows it to maintain continuity. Connections aren't bound to IP-port pairs and don’t require a full reconnect. Although `quic-go` doesn’t yet support active migration, passive rebinding already works in practice.

Because QUIC encrypts its transport metadata, it’s also more robust against middlebox interference. Encrypted packet numbers, ACKs, and control frames reduce the risk of unintended behavior by on-path devices, which can degrade TCP performance.

### Congestion Control Flexibility

Finally, QUIC enables pluggable congestion control. The protocol doesn’t prescribe one algorithm—BBR, Cubic, and custom logic are all possible at the application layer. This allows fine-tuning behavior for different latency and throughput tradeoffs.

## 0-RTT Connections

QUIC supports 0-RTT handshakes, allowing clients to send application data during the initial handshake on repeat connections. This reduces startup latency significantly. However, because 0-RTT data can be replayed by an attacker, it must be used carefully—typically limited to idempotent operations and trusted clients.
{ .annotate }

1. A replay attack occurs when an attacker captures valid network data—such as a request or handshake—and maliciously retransmits it to trick the server into executing it again. In the context of 0-RTT, since early data is sent before the handshake completes, it can be replayed by an adversary on a different connection, potentially causing duplicated actions (like double-purchasing or unauthorized state changes). This is why 0-RTT data must be idempotent or explicitly protected against replay.

```go
{%
    include-markdown "02-networking/src/quic_client.go"
    start="// DialAddrEarly-start"
    end="// DialAddrEarly-end"
%}
```

0-RTT is particularly beneficial for latency-sensitive applications like gaming, VoIP, and real-time financial data feeds.

??? example "Show the complete 0-RTT Server/Client examples"
	??? example "0-RTT Server"
	    ```go
	    {% include "02-networking/src/quic_server.go" %}
	    ```

	??? example "0-RTT Client"
	    ```go
	    {% include "02-networking/src/quic_client.go" %}
	    ```

	**Expected Output**

	When the server is started and the client is executed immediately afterward, you should see:

	***Server Console***
	```text
	QUIC server listening on localhost:4242
	2025/06/23 16:33:00 Received: Hello over 0-RTT
	```

	***Client Console***
	```text
	0-RTT client sent: Hello over 0-RTT
	```

This confirms that early data was transmitted and accepted during the 0-RTT phase of a resumed session, without waiting for the full handshake to complete.

## Final Thoughts on QUIC with Go

QUIC is a transformative protocol with significant design advantages over TCP and HTTP/2, especially in the context of mobile-first and real-time systems. Its ability to multiplex streams without head-of-line blocking, reduce handshake latency through 0-RTT, and recover gracefully from packet loss makes it particularly effective in environments with unstable connectivity—such as LTE, Wi-Fi roaming, or satellite uplinks.

While the Go ecosystem benefits from quic-go as a mature userspace implementation, it's important to understand current limitations. Most notably, quic-go does not yet support full active connection migration as described in [RFC 9000](https://datatracker.ietf.org/doc/html/rfc9000). Although it handles NAT rebinding passively—maintaining sessions across address changes within the same network—it lacks path validation and interface-switching logic required for full multi-homing or roaming support.

In parallel, a Linux kernel implementation of QUIC is under active development, aiming to provide native support for the protocol alongside TCP and UDP. This effort, [led by Lucien Xin](https://lwn.net/ml/all/cover.1725935420.git.lucien.xin@gmail.com/), proposes a complete QUIC stack inside the kernel, including kTLS integration and socket-based API compatibility. If adopted, this would unlock new performance ceilings for QUIC under high load, bypassing userspace copy overhead and reducing syscall costs for data plane operations.

In short, QUIC’s architecture is well-positioned to outperform legacy transports—especially in variable network conditions. While quic-go already enables many of these benefits, it’s worth keeping in mind what’s implemented today vs. what’s defined by the spec. As ecosystem support deepens—from kernel integration to advanced path management—QUIC’s full potential will become more accessible to systems operating at the edge of latency, reliability, and mobility.

Using QUIC via the quic-go library gives developers access to a transport layer designed for modern network demands. Its built-in stream multiplexing, fast connection setup with 0-RTT, and ability to handle network path changes make it a strong fit for real-time systems and mobile applications where latency and reliability are critical.