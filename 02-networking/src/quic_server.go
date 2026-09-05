package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"

	"github.com/quic-go/quic-go"
)

func main() {
	// quic-server-init-start
	listener, err := quic.ListenAddr("localhost:4242", generateTLSConfig(), nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("QUIC server listening on localhost:4242")

	for {
		conn, err := listener.Accept(context.Background())
		if err != nil {
			log.Println("Accept error:", err)
			continue
		}
		go handleConn(conn)
	}
	// quic-server-init-end
}

func handleConn(conn quic.Connection) {
	// quic-server-handle-start
	defer conn.CloseWithError(0, "bye")

	for {
		stream, err := conn.AcceptStream(context.Background())
		if err != nil {
			return
		}

		go func(s quic.Stream) {
			defer s.Close()

			data, err := io.ReadAll(s)
			if len(data) > 0 {
			    log.Printf("Received: %s", string(data))
			}
			if err != nil && err != io.EOF {
			    if appErr, ok := err.(*quic.ApplicationError); !ok || appErr.ErrorCode != 0 {
			        log.Println("read error:", err)
			    }
			}
		}(stream)
	}
	// quic-server-handle-end
}

func generateTLSConfig() *tls.Config {
	cert, err := tls.LoadX509KeyPair("cert.pem", "key.pem")
	if err != nil {
		log.Fatal(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"quic-0rtt-example"},
	}
}