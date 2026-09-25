// Package networking — Protobuf dispatcher for the UDP Game Server.
//
// Spec 19: the only inbound message type is JoinRaceRequest. The
// dispatcher validates the game_token via the shared HMAC secret +
// the local matchmaking assignments table and returns a
// JoinRaceResponse. Future specs (race loop, input, snapshots) will
// extend this with player+session state.
package networking

import (
	"context"
	"log/slog"
	"net"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	"github.com/gustavo/racing-game-backend/internal/matchmaking"
	"github.com/gustavo/racing-game-backend/protocol/gamepb"
)

// SessionStore is the Game Server's local view of an active player.
// Implemented by a future in-memory map; for the MVP we only need
// the join side.
type SessionStore interface {
	AddPlayer(playerID, raceID uuid.UUID, addr *net.UDPAddr) (initialTick uint64, err error)
}

// Verifier is the dependency the dispatcher needs from the
// matchmaking package. The interface keeps the dispatcher testable
// without pulling in a live Postgres.
type Verifier interface {
	VerifyAndConsume(ctx context.Context, matchID, playerID uuid.UUID, rawToken string) error
}

// Dispatcher routes a decoded Protobuf message to its handler.
type Dispatcher struct {
	verifier Verifier
	sessions SessionStore
	log      *slog.Logger
	rng      func() uint64
}

// NewDispatcher wires a dispatcher.
func NewDispatcher(v Verifier, s SessionStore, log *slog.Logger) *Dispatcher {
	if log == nil {
		log = slog.Default()
	}
	return &Dispatcher{verifier: v, sessions: s, log: log}
}

// Handle turns a raw payload (assumed to be a JoinRaceRequest today)
// into a reply payload (always a JoinRaceResponse).
func (d *Dispatcher) Handle(ctx context.Context, payload []byte, from *net.UDPAddr) []byte {
	req := &gamepb.JoinRaceRequest{}
	if err := proto.Unmarshal(payload, req); err != nil {
		d.log.Debug("dispatcher.unmarshal.error", slog.String("err", err.Error()), slog.String("from", from.String()))
		return marshalJoinResp(false, "bad request", "", 0)
	}

	matchID, err := uuid.Parse(req.GetMatchId())
	if err != nil {
		return marshalJoinResp(false, "bad match_id", "", 0)
	}
	playerID, err := uuid.Parse(req.GetPlayerId())
	if err != nil {
		return marshalJoinResp(false, "bad player_id", "", 0)
	}
	rawToken := req.GetGameToken()
	if rawToken == "" {
		return marshalJoinResp(false, "missing token", "", 0)
	}

	// The verifier (matchmaking.Repo with the same HMAC secret) must
	// validate the signature, expiry, and existence in
	// matchmaking_assignments, and flip consumed=true on success.
	if err := d.verifier.VerifyAndConsume(ctx, matchID, playerID, rawToken); err != nil {
		d.log.Info("dispatcher.join.rejected",
			slog.String("match_id", matchID.String()),
			slog.String("player_id", playerID.String()),
			slog.String("err", err.Error()),
			slog.String("from", from.String()),
		)
		return marshalJoinResp(false, err.Error(), "", 0)
	}

	raceID := uuid.New()
	initialTick := uint64(time.Now().UnixMilli()) & 0xFFFFFFFF
	if d.sessions != nil {
		it, err := d.sessions.AddPlayer(playerID, raceID, from)
		if err != nil {
			return marshalJoinResp(false, "session error", "", 0)
		}
		if it != 0 {
			initialTick = it
		}
	}

	d.log.Info("dispatcher.join.ok",
		slog.String("match_id", matchID.String()),
		slog.String("player_id", playerID.String()),
		slog.String("race_id", raceID.String()),
		slog.String("from", from.String()),
	)
	return marshalJoinResp(true, "", raceID.String(), initialTick)
}

func marshalJoinResp(ok bool, reason, raceID string, tick uint64) []byte {
	r := &gamepb.JoinRaceResponse{Ok: ok, Error: reason, RaceId: raceID, InitialTick: tick}
	out, err := proto.Marshal(r)
	if err != nil {
		// Marshal of a JoinRaceResponse can only fail in extreme cases
		// (OOM); return a stub so the client does not hang.
		return []byte{0x08, 0x00} // ok=false, error=""
	}
	return out
}

// IsRetryable reports whether an error from the verifier (or any other
//   caller) can be sent back to the Unity client as-is, or if it
//   should be redacted. Matchmaking sentinel errors map to friendly
//   phrases; the rest fall through as-is.
func IsRetryable(err error) bool {
	return err == nil ||
		err == matchmaking.ErrTokenInvalid ||
		err == matchmaking.ErrTokenExpired ||
		err == matchmaking.ErrTokenConsumed
}
