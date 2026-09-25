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
	"github.com/gustavo/racing-game-backend/internal/race"
	"github.com/gustavo/racing-game-backend/protocol/gamepb"
)

type stubVerifier struct {
	err  error
	last struct {
		match, player uuid.UUID
		token         string
	}
}

func (s *stubVerifier) VerifyAndConsume(_ context.Context, matchID, playerID uuid.UUID, raw string) error {
	s.last.match, s.last.player, s.last.token = matchID, playerID, raw
	return s.err
}

type stubRaceMgr struct {
	r *race.Race
}

func (s *stubRaceMgr) RegisterPlayer(_ context.Context, matchID, playerID uuid.UUID) (*race.Race, error) {
	if s.r == nil {
		return nil, errors.New("no race")
	}
	s.r.AddPlayer(playerID, race.DefaultSpec())
	return s.r, nil
}
func (s *stubRaceMgr) PlayerRace(_ uuid.UUID) (*race.Race, bool) {
	if s.r == nil {
		return nil, false
	}
	return s.r, true
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

func encodeInput(t *testing.T, in *gamepb.PlayerInput) []byte {
	t.Helper()
	out, err := proto.Marshal(in)
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

func newStack(t *testing.T) (*Dispatcher, *matchmaking.Repo, *race.Race, *SessionRegistry, *stubVerifier) {
	t.Helper()
	// stubRaceMgr wraps a fresh race; the race will not actually start
	// its tick loop because we never call Run.
	track := &race.Track{
		ID:        uuid.New(),
		Name:      "Test",
		Waypoints: []race.Waypoint{{X: 0, Z: 0}, {X: 5, Z: 0}, {X: 5, Z: 5}, {X: 0, Z: 5}},
	}
	r := race.New(uuid.New(), track, 30, 1, 4, newLogger())
	verifier := &stubVerifier{}
	sessions := NewSessionRegistry()
	mgr := &stubRaceMgr{r: r}
	d := NewDispatcher(verifier, mgr, sessions, newLogger())
	return d, nil, r, sessions, verifier
}

func TestDispatcherHappyPath(t *testing.T) {
	d, _, r, _, v := newStack(t)
	matchID := uuid.New()
	playerID := uuid.New()
	req := &gamepb.JoinRaceRequest{
		MatchId: matchID.String(), PlayerId: playerID.String(), GameToken: "tok",
	}
	from := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 7001}
	reply := d.Handle(context.Background(), encodeJoinReq(t, req), from)
	r2 := decodeJoinResp(t, reply)
	if !r2.GetOk() {
		t.Fatalf("expected ok=true, got %+v", r2)
	}
	if r2.GetRaceId() != r.ID.String() {
		t.Fatalf("race id mismatch: %s vs %s", r2.GetRaceId(), r.ID)
	}
	if v.last.match != matchID {
		t.Fatal("verifier not called")
	}
}

func TestDispatcherRejectsBadToken(t *testing.T) {
	d2 := &Dispatcher{verifier: &stubVerifier{err: matchmaking.ErrTokenInvalid},
		log: newLogger(), sessions: NewSessionRegistry(),
		races: &stubRaceMgr{},
	}
	req := &gamepb.JoinRaceRequest{
		MatchId: uuid.New().String(), PlayerId: uuid.New().String(), GameToken: "x",
	}
	reply := d2.Handle(context.Background(), encodeJoinReq(t, req),
		&net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	r2 := decodeJoinResp(t, reply)
	if r2.GetOk() {
		t.Fatal("expected ok=false")
	}
}

func TestDispatcherRejectsBadUUID(t *testing.T) {
	d, _, _, _, _ := newStack(t)
	req := &gamepb.JoinRaceRequest{
		MatchId: "not-a-uuid", PlayerId: uuid.New().String(), GameToken: "x",
	}
	reply := d.Handle(context.Background(), encodeJoinReq(t, req),
		&net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	r2 := decodeJoinResp(t, reply)
	if r2.GetOk() || r2.GetError() == "" {
		t.Fatalf("expected rejection, got %+v", r2)
	}
}

func TestDispatcherRejectsMissingToken(t *testing.T) {
	d, _, _, _, _ := newStack(t)
	req := &gamepb.JoinRaceRequest{
		MatchId: uuid.New().String(), PlayerId: uuid.New().String(), GameToken: "",
	}
	reply := d.Handle(context.Background(), encodeJoinReq(t, req),
		&net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	r2 := decodeJoinResp(t, reply)
	if r2.GetOk() {
		t.Fatal("expected ok=false for empty token")
	}
}

func TestDispatcherReplayAfterConsume(t *testing.T) {
	v := &stubVerifier{}
	track := &race.Track{
		ID:        uuid.New(),
		Name:      "Test",
		Waypoints: []race.Waypoint{{X: 0, Z: 0}, {X: 5, Z: 0}},
	}
	r := race.New(uuid.New(), track, 30, 1, 4, newLogger())
	sessions := NewSessionRegistry()
	d := NewDispatcher(v, &stubRaceMgr{r: r}, sessions, newLogger())

	matchID := uuid.New()
	playerID := uuid.New()
	req := &gamepb.JoinRaceRequest{
		MatchId: matchID.String(), PlayerId: playerID.String(), GameToken: "tok",
	}
	from := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 7001}
	d.Handle(context.Background(), encodeJoinReq(t, req), from)
	v.err = matchmaking.ErrTokenConsumed
	reply := d.Handle(context.Background(), encodeJoinReq(t, req), from)
	r2 := decodeJoinResp(t, reply)
	if r2.GetOk() {
		t.Fatal("expected ok=false on replay")
	}
}

func TestDispatcherEnqueuesPlayerInputAfterJoin(t *testing.T) {
	v := &stubVerifier{}
	track := &race.Track{
		ID:        uuid.New(),
		Name:      "Test",
		Waypoints: []race.Waypoint{{X: 0, Z: 0}, {X: 5, Z: 0}},
	}
	r := race.New(uuid.New(), track, 30, 1, 4, newLogger())
	sessions := NewSessionRegistry()
	d := NewDispatcher(v, &stubRaceMgr{r: r}, sessions, newLogger())

	matchID := uuid.New()
	playerID := uuid.New()
	req := &gamepb.JoinRaceRequest{
		MatchId: matchID.String(), PlayerId: playerID.String(), GameToken: "tok",
	}
	from := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 7001}
	d.Handle(context.Background(), encodeJoinReq(t, req), from)

	// Now send a PlayerInput from the same addr.
	in := &gamepb.PlayerInput{Sequence: 1, Throttle: 1, Brake: 0, Steering: 0}
	out := d.Handle(context.Background(), encodeInput(t, in), from)
	if out != nil {
		t.Fatalf("expected nil reply for input, got %x", out)
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
