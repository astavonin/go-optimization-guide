package main

import (
	"context"
	"crypto/tls"
	"log"
	"time"

	"github.com/quic-go/quic-go"
)

func main() {
	sessionCache := tls.NewLRUClientSessionCache(128)
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		EnableActiveMigration: true,
		ClientSessionCache: sessionCache,
		NextProtos:         []string{"quic-0rtt-example"},
	}

	// 1. Establish initial connection for session priming
	conn, err := quic.DialAddr(context.Background(), "localhost:4242", tlsConf, nil)
	if err != nil {
		log.Fatal(err)
	}
	conn.CloseWithError(0, "primed")
	time.Sleep(500 * time.Millisecond)

	// 2. 0-RTT connection
	// DialAddrEarly-start
	earlyConn, err := quic.DialAddrEarly(context.Background(), "localhost:4242", tlsConf, nil)
	// DialAddrEarly-end
	if err != nil {
		log.Fatal("0-RTT failed:", err)
	}
	stream, err := earlyConn.OpenStreamSync(context.Background())
	if err != nil {
		log.Fatal("stream open failed:", err)
	}

	message := "Hello over 0-RTT"
	if _, err := stream.Write([]byte(message)); err != nil {
		log.Fatal("write failed:", err)
	}
	// Wait briefly to ensure data is sent before closing
	time.Sleep(200 * time.Millisecond)

	stream.Close()
	earlyConn.CloseWithError(0, "done")
	log.Println("0-RTT client sent:", message)
}