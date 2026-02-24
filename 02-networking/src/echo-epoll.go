package main

import (
	"log"
	"net"
	"sync"
	"syscall"
)

func main() {
	// Create an epoll file descriptor.
	epfd, err := syscall.EpollCreate1(0)
	if err != nil {
		log.Fatal("EpollCreate1 error:", err)
	}
	defer syscall.Close(epfd)

	// Start listening on port 9000.
	ln, err := net.Listen("tcp", ":9000")
	if err != nil {
		log.Fatal("Listen error:", err)
	}
	defer ln.Close()

	// Use sync.Map to store the mapping from file descriptor to connection.
	var conns sync.Map // key: int, value: net.Conn

	// Accept new connections in a separate goroutine.
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				log.Println("Accept error:", err)
				continue
			}

			// Assert the connection as a TCP connection.
			tcpConn, ok := conn.(*net.TCPConn)
			if !ok {
				conn.Close()
				continue
			}

			// Obtain the raw connection to extract the file descriptor.
			rawConn, err := tcpConn.SyscallConn()
			if err != nil {
				log.Println("SyscallConn error:", err)
				conn.Close()
				continue
			}

			var fd int
			err = rawConn.Control(func(f uintptr) {
				fd = int(f)
			})
			if err != nil {
				log.Println("Control error:", err)
				conn.Close()
				continue
			}

			// Set the file descriptor to non-blocking mode.
			if err = syscall.SetNonblock(fd, true); err != nil {
				log.Println("SetNonblock error:", err)
				conn.Close()
				continue
			}

			// Register the file descriptor with epoll for read events.
			event := &syscall.EpollEvent{
				Events: syscall.EPOLLIN,
				Fd:     int32(fd),
			}
			if err := syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, fd, event); err != nil {
				log.Println("EpollCtl error:", err)
				conn.Close()
				continue
			}

			// Save the connection in our sync.Map.
			conns.Store(fd, conn)
		}
	}()

	// Buffer for epoll events and for reading data.
	events := make([]syscall.EpollEvent, 128)
	readBuf := make([]byte, 4096)

	// Event loop.
	for {
		n, err := syscall.EpollWait(epfd, events, -1)
		if err != nil {
			if err == syscall.EINTR {
				continue
			}
			log.Fatal("EpollWait error:", err)
		}

		// Process each event.
		for i := 0; i < n; i++ {
			fd := int(events[i].Fd)

			// Retrieve the connection for this fd.
			value, ok := conns.Load(fd)
			if !ok {
				// Connection was removed.
				continue
			}
			conn := value.(net.Conn)

			// Read available data from the connection.
			nread, err := syscall.Read(fd, readBuf)
			if err != nil {
				// If no data is available, try again.
				if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
					continue
				}
				log.Println("Read error on fd", fd, err)
				syscall.EpollCtl(epfd, syscall.EPOLL_CTL_DEL, fd, nil)
				conn.Close()
				conns.Delete(fd)
				continue
			}
			// A zero-byte read indicates that the client closed the connection.
			if nread == 0 {
				syscall.EpollCtl(epfd, syscall.EPOLL_CTL_DEL, fd, nil)
				conn.Close()
				conns.Delete(fd)
				continue
			}

			// Write the response back to the client.
			// In production you may need to handle partial writes and buffer remaining data.
			nwritten, err := syscall.Write(fd, readBuf)
			if err != nil {
				if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
					continue
				}
				log.Println("Write error on fd", fd, err)
				syscall.EpollCtl(epfd, syscall.EPOLL_CTL_DEL, fd, nil)
				conn.Close()
				conns.Delete(fd)
				continue
			}
			if nwritten < len(response) {
				// Partial write handling can be implemented as needed.
			}
		}
	}
}