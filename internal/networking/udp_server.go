// Package networking provides the UDP networking primitives used by the
// Game Server. It is intentionally small: a thin wrapper around
// net.UDPConn plus a pure HandlePacket function whose protocol can be
// swapped out (text today, Protobuf in a future milestone) without
// touching the read loop.
package networking

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"
)

// HandlePacket inspects an incoming datagram and returns the reply to
// send back, or nil when the packet should be silently ignored.
//
// MVP behavior (Spec 08):
//   - "PING" (trimmed)   → "PONG"
//   - anything else      → nil
//
// This is the single point where the wire protocol will be replaced
// by Protobuf decoding in the future.
func HandlePacket(payload []byte) []byte {
	msg := strings.TrimSpace(string(payload))
	if strings.EqualFold(msg, "PING") {
		return []byte("PONG")
	}
	return nil
}

// UDPServer wraps a single net.UDPConn with a context-aware read loop
// and a sync.WaitGroup so callers can wait for shutdown.
type UDPServer struct {
	conn *net.UDPConn
	log  *slog.Logger

	wg   sync.WaitGroup
	done chan struct{}
}

// NewUDPServer binds a UDP socket on the configured port and returns
// a ready-to-run server. The caller must invoke Serve to start the
// read loop.
func NewUDPServer(port int, log *slog.Logger) (*UDPServer, error) {
	if log == nil {
		return nil, errors.New("networking: nil logger")
	}
	addr := &net.UDPAddr{IP: net.IPv4zero, Port: port}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen udp :%d: %w", port, err)
	}
	return &UDPServer{
		conn: conn,
		log:  log,
		done: make(chan struct{}),
	}, nil
}

// LocalAddr returns the actual local address the server is bound to
// (useful when port 0 was used for testing).
func (s *UDPServer) LocalAddr() *net.UDPAddr { return s.conn.LocalAddr().(*net.UDPAddr) }

// Serve runs the read loop until ctx is canceled or the socket is
// closed. Replies are sent synchronously on the same goroutine; for
// the MVP traffic this is fine. A future milestone may add a write
// pool per race.
func (s *UDPServer) Serve(ctx context.Context) error {
	s.wg.Add(1)
	defer s.wg.Done()

	buf := make([]byte, 1500) // typical MTU
	for {
		select {
		case <-ctx.Done():
			s.log.Info("udp server: context canceled, stopping read loop")
			return nil
		case <-s.done:
			return nil
		default:
		}

		// Set a read deadline so we periodically wake up to check
		// the context — otherwise a blocking Read could outlive a
		// shutdown signal.
		_ = s.conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))

		n, from, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			var nerr net.Error
			if errors.As(err, &nerr) && nerr.Timeout() {
				continue
			}
			// "use of closed network connection" is the normal
			// shutdown signal after Close().
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			s.log.Error("udp read error", slog.String("error", err.Error()))
			continue
		}

		// Defensive copy because HandlePacket (and future Protobuf
		// decode) may keep a reference to the slice.
		pkt := make([]byte, n)
		copy(pkt, buf[:n])

		s.log.Debug("udp packet received",
			slog.String("remote", from.String()),
			slog.Int("bytes", n),
		)

		reply := HandlePacket(pkt)
		if reply == nil {
			s.log.Debug("udp packet ignored (no reply)",
				slog.String("remote", from.String()),
			)
			continue
		}

		if _, err := s.conn.WriteToUDP(reply, from); err != nil {
			s.log.Error("udp write error",
				slog.String("remote", from.String()),
				slog.String("error", err.Error()),
			)
			continue
		}

		s.log.Info("udp packet replied",
			slog.String("remote", from.String()),
			slog.String("reply", string(reply)),
		)
	}
}

// Close shuts down the socket and waits for the read loop to exit.
// Safe to call multiple times.
func (s *UDPServer) Close() error {
	select {
	case <-s.done:
		// already closed
	default:
		close(s.done)
	}
	err := s.conn.Close()
	s.wg.Wait()
	return err
}