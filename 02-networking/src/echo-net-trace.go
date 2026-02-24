package main

import (
    "crypto/sha256"
    "encoding/hex"

	"bufio"
	"log"
	"net"
	"os"
	"runtime/trace"
	"sync/atomic"
	"time"
)

func hash(s string) string {
    h := sha256.Sum256([]byte(s))
    return hex.EncodeToString(h[:])
}

var activeConns int32

func handle(conn net.Conn) {
	defer conn.Close()
	atomic.AddInt32(&activeConns, 1)
	defer atomic.AddInt32(&activeConns, -1)

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	const flushInterval = 10
	count := 0

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("Connection closed (%s): %v", conn.RemoteAddr(), err)
			return
		}
		hash(line)
		_, err = writer.WriteString(line)
		if err != nil {
			log.Printf("Write failed (%s): %v", conn.RemoteAddr(), err)
			return
		}
		count++
		if count >= flushInterval {
			if err := writer.Flush(); err != nil {
				log.Printf("Flush failed (%s): %v", conn.RemoteAddr(), err)
				return
			}
			count = 0
		}
	}
}

func main() {
	// Setup trace output
	traceFile, err := os.Create("trace.out")
	if err != nil {
		log.Fatalf("failed to create trace file: %v", err)
	}
	defer traceFile.Close()

	if err := trace.Start(traceFile); err != nil {
		log.Fatalf("failed to start trace: %v", err)
	}
	defer trace.Stop()

	ln, err := net.Listen("tcp", ":9000")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	log.Println("Listening on :9000")

	// Periodic connection count logger
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			log.Printf("Active connections: %d\n", atomic.LoadInt32(&activeConns))
		}
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			continue
		}
		go handle(conn)
	}
}