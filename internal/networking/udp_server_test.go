package networking

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"testing"
	"time"
)

// TestHandlePacketPING verifies the canonical PING → PONG behavior.
func TestHandlePacketPING(t *testing.T) {
	cases := []string{"PING", "PING\n", "  ping  ", "pInG"}
	for _, in := range cases {
		got := HandlePacket([]byte(in))
		if !bytes.Equal(got, []byte("PONG")) {
			t.Errorf("HandlePacket(%q) = %q, want PONG", in, got)
		}
	}
}

// TestHandlePacketUnknownReturnsNil ensures unknown payloads are not
// replied to.
func TestHandlePacketUnknownReturnsNil(t *testing.T) {
	if got := HandlePacket([]byte("hello")); got != nil {
		t.Errorf("HandlePacket(hello) = %q, want nil", got)
	}
	if got := HandlePacket([]byte{}); got != nil {
		t.Errorf("HandlePacket(empty) = %q, want nil", got)
	}
}

// TestUDPServerPINGPONGEndToEnd spins up the server on an ephemeral
// port and exercises the full PING/PONG round trip.
func TestUDPServerPINGPONGEndToEnd(t *testing.T) {
	if os.Getenv("CI_SKIP_UDP_E2E") != "" {
		t.Skip("CI_SKIP_UDP_E2E set")
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	srv, err := NewUDPServer(0, log)
	if err != nil {
		t.Fatalf("NewUDPServer: %v", err)
	}
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = srv.Serve(ctx) }()

	client, err := net.DialUDP("udp", nil, srv.LocalAddr())
	if err != nil {
		t.Fatalf("DialUDP: %v", err)
	}
	defer client.Close()
	_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))

	if _, err := client.Write([]byte("PING")); err != nil {
		t.Fatalf("Write PING: %v", err)
	}

	buf := make([]byte, 16)
	n, err := client.Read(buf)
	if err != nil {
		t.Fatalf("Read reply: %v", err)
	}
	if got := string(buf[:n]); got != "PONG" {
		t.Errorf("reply = %q, want PONG", got)
	}
}

// TestNewUDPServerRejectsNilLogger exercises the defensive nil guard.
func TestNewUDPServerRejectsNilLogger(t *testing.T) {
	if _, err := NewUDPServer(0, nil); err == nil {
		t.Fatal("NewUDPServer(nil log) should error")
	}
}

// TestUDPServerCloseIsIdempotent ensures Close can be called twice.
func TestUDPServerCloseIsIdempotent(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	srv, err := NewUDPServer(0, log)
	if err != nil {
		t.Fatalf("NewUDPServer: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = srv.Serve(ctx) }()

	if err := srv.Close(); err != nil {
		t.Errorf("first Close: %v", err)
	}
	if err := srv.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		// Second close returns "use of closed network connection",
		// which we treat as benign here.
		t.Errorf("second Close returned unexpected error: %v", err)
	}
}