// Command udp-client is a developer utility that sends a PING to the
// Game Server over UDP and prints the PONG reply.
//
// Usage:
//
//	go run ./cmd/udp-client                # localhost:7000
//	go run ./cmd/udp-client --host=1.2.3.4 --port=7000
//	go run ./cmd/udp-client --timeout=5s
//
// Exit codes:
//
//	0  PONG received
//	1  timeout or network error
package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"time"
)

func main() {
	host := flag.String("host", "localhost", "Game Server host")
	port := flag.Int("port", 7000, "Game Server UDP port")
	timeout := flag.Duration("timeout", 3*time.Second, "Read deadline")
	flag.Parse()

	addr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(*host, fmt.Sprintf("%d", *port)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "udp-client: resolve %s:%d: %v\n", *host, *port, err)
		os.Exit(1)
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "udp-client: dial %s: %v\n", addr, err)
		os.Exit(1)
	}
	defer conn.Close()

	if err := conn.SetReadDeadline(time.Now().Add(*timeout)); err != nil {
		fmt.Fprintf(os.Stderr, "udp-client: set deadline: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Sent: PING → %s\n", addr)
	if _, err := conn.Write([]byte("PING")); err != nil {
		fmt.Fprintf(os.Stderr, "udp-client: write PING: %v\n", err)
		os.Exit(1)
	}

	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "udp-client: read reply: %v\n", err)
		os.Exit(1)
	}

	reply := string(buf[:n])
	fmt.Printf("Received: %s\n", reply)
	if reply != "PONG" {
		fmt.Fprintf(os.Stderr, "udp-client: unexpected reply %q, want PONG\n", reply)
		os.Exit(1)
	}
}