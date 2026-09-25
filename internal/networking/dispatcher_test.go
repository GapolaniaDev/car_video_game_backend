package networking

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	"github.com/gustavo/racing-game-backend/internal/matchmaking"
	"github.com/gustavo/racing-game-backend/protocol/gamepb"
)

// stubVerifier records the last (match, player, token) and returns
// whatever error was preloaded. It also implements MarkConsumed by
// recording a "used" flag.
type stubVerifier struct {
	err  error
	last *struct {
		match, player uuid.UUID
		token         string
	}
	used bool
}

func (s *stubVerifier) VerifyAndConsume(_ context.Context, matchID, playerID uuid.UUID, raw string) error {
	s.last = &struct {
		match, player uuid.UUID
		token         string
	}{matchID, playerID, raw}
	if s.err != nil {
		return s.err
	}
	s.used = true
	return nil
}

// unused, kept for a future spec that needs SessionStore
type noopSessions struct{}

func (n noopSessions) AddPlayer(_, _ uuid.UUID, _ *net.UDPAddr) (uint64, error) {
	return 0, nil
}

func newLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func encodeJoinReq(t *testing.T, req *gamepb.JoinRaceRequest) []byte {
	t.Helper()
	out, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return out
}

func decodeJoinResp(t *testing.T, payload []byte) *gamepb.JoinRaceResponse {
	t.Helper()
	r := &gamepb.JoinRaceResponse{}
	if err := proto.Unmarshal(payload, r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return r
}

func TestDispatcherHappyPath(t *testing.T) {
	v := &stubVerifier{}
	d := NewDispatcher(v, nil, newLogger())
	matchID := uuid.New()
	playerID := uuid.New()
	req := &gamepb.JoinRaceRequest{
		MatchId:   matchID.String(),
		PlayerId:  playerID.String(),
		GameToken: "any.thing",
	}
	reply := d.Handle(context.Background(), encodeJoinReq(t, req), &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 7000})
	if reply == nil {
		t.Fatal("nil reply")
	}
	r := decodeJoinResp(t, reply)
	if !r.GetOk() {
		t.Fatalf("expected ok=true, got %+v", r)
	}
	if r.GetRaceId() == "" {
		t.Fatal("expected race_id, got empty")
	}
	if !v.used {
		t.Fatal("verifier was not called")
	}
}

func TestDispatcherRejectsBadToken(t *testing.T) {
	v := &stubVerifier{err: matchmaking.ErrTokenInvalid}
	d := NewDispatcher(v, nil, newLogger())
	req := &gamepb.JoinRaceRequest{
		MatchId: uuid.New().String(), PlayerId: uuid.New().String(), GameToken: "x",
	}
	reply := d.Handle(context.Background(), encodeJoinReq(t, req), &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 7000})
	r := decodeJoinResp(t, reply)
	if r.GetOk() {
		t.Fatal("expected ok=false")
	}
	if !errors.Is(matchmaking.ErrTokenInvalid, matchmaking.ErrTokenInvalid) { // sanity
		t.Fatal("sentinel mismatch")
	}
}

func TestDispatcherRejectsBadUUID(t *testing.T) {
	v := &stubVerifier{}
	d := NewDispatcher(v, nil, newLogger())
	req := &gamepb.JoinRaceRequest{
		MatchId: "not-a-uuid", PlayerId: uuid.New().String(), GameToken: "x",
	}
	reply := d.Handle(context.Background(), encodeJoinReq(t, req), &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 7000})
	r := decodeJoinResp(t, reply)
	if r.GetOk() || r.GetError() == "" {
		t.Fatalf("expected rejection, got %+v", r)
	}
}

func TestDispatcherRejectsMissingToken(t *testing.T) {
	v := &stubVerifier{}
	d := NewDispatcher(v, nil, newLogger())
	req := &gamepb.JoinRaceRequest{
		MatchId: uuid.New().String(), PlayerId: uuid.New().String(), GameToken: "",
	}
	reply := d.Handle(context.Background(), encodeJoinReq(t, req), &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 7000})
	r := decodeJoinResp(t, reply)
	if r.GetOk() {
		t.Fatal("expected ok=false for empty token")
	}
}

func TestDispatcherRejectsReplayAfterConsume(t *testing.T) {
	v := &stubVerifier{}
	d := NewDispatcher(v, nil, newLogger())
	req := &gamepb.JoinRaceRequest{
		MatchId: uuid.New().String(), PlayerId: uuid.New().String(), GameToken: "tok",
	}
	d.Handle(context.Background(), encodeJoinReq(t, req), &net.UDPAddr{})
	v.err = matchmaking.ErrTokenConsumed
	reply := d.Handle(context.Background(), encodeJoinReq(t, req), &net.UDPAddr{})
	r := decodeJoinResp(t, reply)
	if r.GetOk() {
		t.Fatal("expected ok=false on replay")
	}
}

func TestIsRetryable(t *testing.T) {
	if !IsRetryable(nil) {
		t.Fatal("nil should be retryable")
	}
	if !IsRetryable(matchmaking.ErrTokenInvalid) {
		t.Fatal("ErrTokenInvalid should be retryable (transparent)")
	}
	if IsRetryable(errors.New("other")) {
		t.Fatal("unknown error should NOT be retryable")
	}
}
