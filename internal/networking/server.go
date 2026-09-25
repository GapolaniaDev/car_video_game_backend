// Package networking provides the UDP networking primitives used by
// the Game Server. It exposes a thin wrapper around net.UDPConn plus
// a Protobuf dispatcher.
package networking

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/gustavo/racing-game-backend/internal/protoframing"
)

// Dispatcher routes a decoded Protobuf message to a reply. It must
// be safe to call concurrently from multiple goroutines.
type DispatcherHandler interface {
	Handle(ctx context.Context, payload []byte, from *net.UDPAddr) []byte
}

// UDPServer wraps a single net.UDPConn with a context-aware read loop
// and a sync.WaitGroup so callers can wait for shutdown.
type UDPServer struct {
	conn *net.UDPConn
	log  *slog.Logger

	dispatcher DispatcherHandler

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

// SetDispatcher wires the Protobuf dispatcher. Must be called before
// Serve.
func (s *UDPServer) SetDispatcher(d DispatcherHandler) { s.dispatcher = d }

// LocalAddr returns the actual local address the server is bound to
// (useful when port 0 was used for testing).
func (s *UDPServer) LocalAddr() *net.UDPAddr { return s.conn.LocalAddr().(*net.UDPAddr) }

// Serve runs the read loop until ctx is canceled or the socket is
// closed. Each datagram is length-prefix decoded, dispatched, and the
// reply is length-prefix encoded back to the remote.
func (s *UDPServer) Serve(ctx context.Context) error {
	s.wg.Add(1)
	defer s.wg.Done()

	buf := make([]byte, 2048)
	var pending []byte
	assembled := make([]byte, 0, 4096)

	for {
		select {
		case <-ctx.Done():
			s.log.Info("udp server: context canceled, stopping read loop")
			return nil
		case <-s.done:
			return nil
		default:
		}

		_ = s.conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))

		n, from, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			var nerr net.Error
			if errors.As(err, &nerr) && nerr.Timeout() {
				continue
			}
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			s.log.Error("udp read error", slog.String("error", err.Error()))
			continue
		}

		// Reassemble one frame's worth of bytes.
		pending = append(pending[:0], buf[:n]...)
		assembled = assembled[:0]

		payload, _, err := protoframing.Decode(pending)
		if err != nil {
			s.log.Debug("udp frame decode error",
				slog.String("from", from.String()),
				slog.String("err", err.Error()),
				slog.Int("bytes", n),
			)
			continue
		}
		assembled = append(assembled, payload...)

		if s.dispatcher == nil {
			s.log.Debug("udp packet received but no dispatcher wired",
				slog.String("from", from.String()),
				slog.Int("bytes", len(assembled)),
			)
			continue
		}

		reply := s.dispatcher.Handle(ctx, assembled, from)
		if len(reply) == 0 {
			continue
		}
		framed, err := protoframing.Encode(reply)
		if err != nil {
			s.log.Error("udp encode reply error", slog.String("err", err.Error()))
			continue
		}
		if _, err := s.conn.WriteToUDP(framed, from); err != nil {
			s.log.Error("udp write error",
				slog.String("remote", from.String()),
				slog.String("error", err.Error()),
			)
			continue
		}
		s.log.Debug("udp packet replied",
			slog.String("remote", from.String()),
			slog.Int("bytes", len(framed)),
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
