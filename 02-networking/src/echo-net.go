package main

import (
    "bufio"
    "fmt"
    "net"
    "time"
)

func main() {
    // Start listening on TCP port 9000
    listener, err := net.Listen("tcp", ":9000")
    if err != nil {
        panic(err) // Exit if the port can't be bound
    }
    fmt.Println("Echo server listening on :9000")

    // Accept incoming connections in a loop
    for {
        conn, err := listener.Accept() // Accept new client connection
        if err != nil {
            fmt.Printf("Accept error: %v\n", err)
            continue // Skip this iteration on error
        }

        // Handle the connection in a new goroutine for concurrency
        go handle(conn)
    }
}

// handle echoes data back to the client line-by-line
func handle(conn net.Conn) {
    defer conn.Close() // Ensure connection is closed on exit

    reader := bufio.NewReader(conn) // Wrap connection with buffered reader

    for {
        // Set a read deadline to avoid hanging goroutines if client disappears
        conn.SetReadDeadline(time.Now().Add(5 * 60 * time.Second)) // 5 minutes timeout

        // Read input until newline character
        line, err := reader.ReadString('\n')
        if err != nil {
            fmt.Printf("Connection closed: %v\n", err)
            return // Exit on read error (e.g. client disconnect)
        }

        // Echo the received line back to the client
        _, err = conn.Write([]byte(line))
        if err != nil {
            fmt.Printf("Write error: %v\n", err)
            return // Exit on write error
        }
    }
}