package networking

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"testing"
	"time"

	"github.com/gustavo/racing-game-backend/internal/protoframing"
)

// echoDispatcher unconditionally replies to whatever it receives,
// prefixing the payload with an "echoed:" header. Used by the
// end-to-end test below.
type echoDispatcher struct{ log *slog.Logger }

func (d echoDispatcher) Handle(_ context.Context, payload []byte, _ *net.UDPAddr) []byte {
	d.log.Debug("echo.handle", "len", len(payload))
	return payload
}

// silenceHandler is a no-op dispatcher so we can verify dropped
// packets are not replied to.
type silenceHandler struct{}

func (silenceHandler) Handle(_ context.Context, _ []byte, _ *net.UDPAddr) []byte { return nil }

// TestUDPServerLengthPrefixedRoundTrip spins up the server on an
// ephemeral port with an echo dispatcher and verifies framing.
func TestUDPServerLengthPrefixedRoundTrip(t *testing.T) {
	if os.Getenv("CI_SKIP_UDP_E2E") != "" {
		t.Skip("CI_SKIP_UDP_E2E set")
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	srv, err := NewUDPServer(0, log)
	if err != nil {
		t.Fatalf("NewUDPServer: %v", err)
	}
	srv.SetDispatcher(echoDispatcher{log: log})
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = srv.Serve(ctx) }()

	// Give the read loop a tick.
	time.Sleep(50 * time.Millisecond)

	client, err := net.DialUDP("udp", nil, srv.LocalAddr())
	if err != nil {
		t.Fatalf("DialUDP: %v", err)
	}
	defer client.Close()
	_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))

	payload := []byte("hello, world")
	framed, err := protoframing.Encode(payload)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if _, err := client.Write(framed); err != nil {
		t.Fatalf("Write: %v", err)
	}

	buf := make([]byte, 256)
	n, err := client.Read(buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	got, _, err := protoframing.Decode(buf[:n])
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if string(got) != string(payload) {
		t.Errorf("echo mismatch: got %q want %q", got, payload)
	}
}

// TestUDPServerSilencesUnknown verifies that with no dispatcher wired
// the server does not reply.
func TestUDPServerSilencesUnknown(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	srv, err := NewUDPServer(0, log)
	if err != nil {
		t.Fatalf("NewUDPServer: %v", err)
	}
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = srv.Serve(ctx) }()
	time.Sleep(50 * time.Millisecond)

	client, err := net.DialUDP("udp", nil, srv.LocalAddr())
	if err != nil {
		t.Fatalf("DialUDP: %v", err)
	}
	defer client.Close()
	_ = client.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	framed, _ := protoframing.Encode([]byte("ignored"))
	_, _ = client.Write(framed)
	buf := make([]byte, 64)
	if _, err := client.Read(buf); err == nil {
		t.Fatal("expected read to time out, got data instead")
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
		t.Errorf("second Close returned unexpected error: %v", err)
	}
}
