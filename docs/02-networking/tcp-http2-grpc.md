# Comparing TCP, HTTP/2, and gRPC Performance in Go

In distributed systems, the choice of communication protocol shapes how services interact under real-world load. It influences not just raw throughput and latency, but also how well the system scales, how much CPU and memory it consumes, and how predictable its behavior remains under pressure. In this article, we dissect three prominent options—raw TCP with custom framing, HTTP/2 via Go's built-in `net/http` package, and gRPC—and explore their performance characteristics through detailed benchmarks and practical scenarios.

## Raw TCP with Custom Framing

Raw TCP provides maximum flexibility with virtually no protocol overhead, but that comes at a cost: all message boundaries, framing logic, and error handling must be implemented manually. Since TCP delivers a continuous byte stream with no inherent notion of messages, applications must explicitly define how to separate and interpret those bytes.

### Custom Framing Protocol

A common way to handle message boundaries over TCP is to use length-prefix framing: each message starts with a 4-byte header that tells the receiver how many bytes to read next. The length is encoded in big-endian format, following the standard network byte order, so it behaves consistently across different systems. This setup solves a core issue with TCP—while it guarantees reliable delivery, it doesn’t preserve message boundaries. Without knowing the size upfront, the receiver has no way to tell where one message ends and the next begins.

TCP guarantees reliable, in-order delivery of bytes, but it does not preserve or indicate message boundaries. For example, if a client sends three logical messages:

```
[msg1][msg2][msg3]
```

the server may receive them as a continuous byte stream with arbitrary segmentations, such as:

```
[msg1_part][msg2][msg3_part]
```

TCP delivers a continuous stream of bytes with no built-in concept of where one message stops and another starts. This means the receiver can’t rely on read boundaries to infer message boundaries—what arrives might be a partial message, multiple messages concatenated, or an arbitrary slice of both. To make sense of structured data over such a stream, the application needs a framing strategy. Length-prefixing does this by including the size of the message up front, so the receiver knows exactly how many bytes to expect before starting to parse the payload.

#### Protocol Structure

While length-prefixing is the most common and efficient framing strategy, there are other options depending on the use case. Other framing strategies exist, each with its own trade-offs in terms of simplicity, robustness, and flexibility. Delimiter-based framing uses a specific byte or sequence—like `\n` or `\0` to signal the end of a message. It’s easy to implement but fragile if the delimiter can appear in the payload. Fixed-size framing avoids ambiguity by making every message the same length, which simplifies parsing and memory allocation but doesn’t work well when message sizes vary. Self-describing formats like Protobuf or ASN.1 embed length and type information inside the payload itself, allowing for richer structure and evolution over time, but require more sophisticated parsing logic and schema awareness on both ends. Choosing the right approach depends on how much control you need, how predictable your data is, and how much complexity you’re willing to absorb.

Each frame of length-prefixing implementation consists of:

```
| Length (4 bytes) | Payload (Length bytes) |
```

* **Length:** A 4-byte unsigned integer encoded in big-endian format (network byte order), representing the number of bytes in the payload.
* **Payload:** Raw binary data of arbitrary length.

The use of `binary.BigEndian.PutUint32` ensures the frame length is encoded in a standardized format—most significant byte first. This is consistent with Internet protocol standards (1) , allowing for predictable decoding and reliable interoperation between heterogeneous systems.
{ .annotate }

1. Following the established convention of network byte order, which is defined as big-endian in [RFC 791, Section 3.1](https://datatracker.ietf.org/doc/html/rfc791#section-3.1) and used consistently in transport and application protocols such as TCP ([RFC 793](https://datatracker.ietf.org/doc/html/rfc793)).

```go
func writeFrame(conn net.Conn, payload []byte) error {
    frameLen := uint32(len(payload))
    buf := make([]byte, 4+len(payload))
    binary.BigEndian.PutUint32(buf[:4], frameLen)
    copy(buf[4:], payload)
    _, err := conn.Write(buf)
    return err
}

func readFrame(conn net.Conn) ([]byte, error) {
    lenBuf := make([]byte, 4)
    if _, err := io.ReadFull(conn, lenBuf); err != nil {
        return nil, err
    }
    frameLen := binary.BigEndian.Uint32(lenBuf)
    payload := make([]byte, frameLen)
    if _, err := io.ReadFull(conn, payload); err != nil {
        return nil, err
    }
    return payload, nil
}
```

This approach is straightforward, performant, and predictable, yet it provides no built-in concurrency management, request multiplexing, or flow control—these must be explicitly managed by the developer.

#### Disadvantages

While the protocol is efficient and minimal, it lacks several features commonly found in more complex transport protocols. The lack of built-in framing features in raw TCP means that key responsibilities shift entirely to the application layer. There’s no support for multiplexing, so only one logical message can be in flight per connection unless additional coordination is built manually—pushing clients to open multiple connections to achieve parallelism. Flow control is also absent; unlike HTTP/2 or gRPC, there’s no way to signal backpressure, making it easy for a fast sender to overwhelm a slow receiver, potentially exhausting memory or triggering a crash. There’s no space for structured metadata like message types, compression flags, or trace context unless you embed them yourself into the payload format. And error handling is purely ad hoc—there’s no protocol-level mechanism for communicating faults, so malformed frames or incorrect lengths often lead to abrupt connection resets or inconsistent state. 

These limitations might be manageable in tightly scoped, high-performance systems where both ends of the connection are under full control and the protocol behavior is well understood. In such environments, the minimal overhead and direct access to the wire can justify the trade-offs. But in broader production contexts—especially those involving multiple teams, evolving requirements, or untrusted clients—they introduce significant risk. Without strict validation, clear framing, and robust error handling, even small inconsistencies can lead to silent corruption, resource leaks, or hard-to-diagnose failures. Building on raw TCP demands both precise engineering and long-term maintenance discipline.

### Performance Insights

* **Latency:** Lowest achievable due to minimal overhead; ideal for latency-critical scenarios like financial trading systems.
* **Throughput:** Excellent, constrained only by network and application-layer handling.
* **CPU/Memory Cost:** Very low, with negligible overhead from protocol management.

## HTTP/2 via net/http

HTTP/2 brought several protocol-level improvements over HTTP/1.1, including multiplexed streams over a single connection, header compression via HPACK, and support for server push. In Go, these features are integrated directly into the `net/http` standard library, which handles connection reuse, stream multiplexing, and concurrency without requiring manual intervention. Unlike raw TCP, where applications must explicitly define message boundaries, HTTP/2 defines them at the protocol level: each request and response is framed using structured `HEADERS` and `DATA` frames and explicitly closed with an `END_STREAM` flag. These frames are handled entirely within Go’s HTTP/2 implementation, so developers interact with complete, logically isolated messages using the standard `http.Request` and `http.ResponseWriter` interfaces. You don’t have to deal with byte streams or worry about where a message starts or ends—by the time a request hits your handler, it’s already been framed and parsed. When you write a response, the runtime takes care of wrapping it up and signaling completion. That frees you up to focus on the logic, not the plumbing, while still getting the performance benefits of HTTP/2 like multiplexing and connection reuse.

### Server Implementation

Beyond framing and multiplexing, HTTP/2 brings a handful of practical advantages that make server code easier to write and faster to run. It handles connection reuse out of the box, applies flow control to avoid overwhelming either side, and compresses headers using `HPACK` to cut down on overhead. Go’s `net/http` stack takes care of all of this behind the scenes, so you get the benefits without needing to wire it up yourself. As a result, developers can build concurrent, efficient servers without managing low-level connection or stream state manually.

```go
func handler(w http.ResponseWriter, r *http.Request) {
    payload, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "invalid request", http.StatusBadRequest)
        return
    }
    defer r.Body.Close()

    // Process payload...

    w.WriteHeader(http.StatusOK)
    w.Write([]byte("processed"))
}

func main() {
    server := &http.Server{
        Addr:    ":8080",
        Handler: http.HandlerFunc(handler),
    }
    log.Fatal(server.ListenAndServeTLS("server.crt", "server.key"))
}
```

!!! info
    Even this is not mentioned explisitly, this code serves HTTP/2 because it uses `ListenAndServeTLS`, which enables TLS-based communication. Go's `net/http` package automatically upgrades connections to HTTP/2 when a client supports it via ALPN (Application-Layer Protocol Negotiation) during the TLS handshake. Since Go 1.6, this upgrade is implicit—no extra configuration is required. The server transparently handles HTTP/2 requests while remaining compatible with HTTP/1.1 clients.

HTTP/2’s multiplexing capability allows multiple independent streams to share a single TCP connection without blocking each other, which significantly improves connection reuse. This reduces the overhead of establishing and managing parallel connections, especially under high concurrency. As a result, latency is lower and throughput more consistent, even when multiple requests are in flight. These traits make HTTP/2 well-suited for general-purpose web services and internal APIs—places where predictable latency, efficient connection reuse, and solid concurrency handling carry more weight than raw protocol minimalism.

### Performance Insights


* **Latency:** Slightly higher than raw TCP because of framing and compression overhead, but stable and consistent thanks to multiplexing and persistent connections.
* **Throughput:** High under concurrent load; stream multiplexing and header compression help sustain performance without opening more sockets.
* **CPU/Memory Cost:** Moderate overhead, mostly due to header processing, TLS encryption, and flow control mechanisms.

## gRPC

gRPC is a high-performance, contract-first RPC framework built on top of HTTP/2, designed for low-latency, cross-language communication between services. It combines streaming-capable transport with strongly typed APIs defined using Protocol Buffers (Protobuf), enabling compact, efficient message serialization and seamless interoperability across platforms. Unlike traditional HTTP APIs, where endpoints are loosely defined by URL patterns and free-form JSON, gRPC enforces strict interface contracts through `.proto` definitions, which serve as both schema and implementation spec. The gRPC toolchain generates client and server code for multiple languages, eliminating manual serialization, improving safety, and standardizing interactions across heterogeneous systems.

gRPC takes advantage of HTTP/2’s core features—stream multiplexing, flow control, and binary framing—to support both one-off RPC calls and full-duplex streaming, all with built-in backpressure. But it goes further than just transport. It bakes in support for deadlines, cancellation, structured metadata, and standardized error reporting, all of which help services communicate clearly and fail predictably. This makes gRPC a solid choice for internal APIs, service meshes, and performance-critical systems where you need efficiency, strong contracts, and reliable behavior under load.

### gRPC Service Definition

A minimal `.proto` file example:

```protobuf
syntax = "proto3";

service EchoService {
  rpc Echo(EchoRequest) returns (EchoResponse);
}

message EchoRequest {
  string message = 1;
}

message EchoResponse {
  string message = 1;
}
```

Generated Go stubs allow developers to easily implement the service:

```go
type server struct {
    UnimplementedEchoServiceServer
}

func (s *server) Echo(ctx context.Context, req *EchoRequest) (*EchoResponse, error) {
    return &EchoResponse{Message: req.Message}, nil
}

func main() {
    lis, err := net.Listen("tcp", ":50051")
    if err != nil {
        log.Fatalf("failed to listen: %v", err)
    }
    grpcServer := grpc.NewServer()
    RegisterEchoServiceServer(grpcServer, &server{})
    grpcServer.Serve(lis)
}
```

### Performance Insights

* **Latency:** Slightly higher than raw HTTP/2 due to additional serialization/deserialization steps, yet still performant for most scenarios.
* **Throughput:** High throughput thanks to efficient payload serialization (protobuf) and inherent HTTP/2 multiplexing capabilities.
* **CPU/Memory Cost:** Higher than HTTP/2 due to protobuf encoding overhead; memory consumption slightly increased due to temporary object allocations.

## Choosing the Right Protocol

- **Internal APIs and microservices**: gRPC usually hits the sweet spot—it’s fast, strongly typed, and easy to work with once the tooling is in place.
- **Low-latency systems and trading platforms:** Raw TCP with custom framing gives you the lowest overhead, but you’re on your own for everything else.
- **Public APIs or general web services:** HTTP/2 via net/http is a solid choice. You get connection reuse, multiplexing, and good performance without needing to pull in a full RPC stack.

Raw TCP gives you maximum control and the best performance on paper—but it also means building everything yourself: framing, flow control, error handling. HTTP/2 and gRPC trade some of that raw speed for built-in structure, better connection handling, and less code to maintain. What’s right depends entirely on where performance matters and how much complexity you want to own.
