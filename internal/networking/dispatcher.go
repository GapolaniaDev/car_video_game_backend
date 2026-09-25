// Package networking — Protobuf dispatcher for the UDP Game Server.
package networking

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	"github.com/gustavo/racing-game-backend/internal/matchmaking"
	"github.com/gustavo/racing-game-backend/internal/race"
	"github.com/gustavo/racing-game-backend/protocol/gamepb"
)

// RaceManager is the subset of race.Manager used by the dispatcher.
type RaceManager interface {
	RegisterPlayer(ctx context.Context, matchID, playerID uuid.UUID) (*race.Race, error)
	PlayerRace(playerID uuid.UUID) (*race.Race, bool)
}

// TokenVerifier is the dependency the dispatcher needs from the
// matchmaking package.
type TokenVerifier interface {
	VerifyAndConsume(ctx context.Context, matchID, playerID uuid.UUID, rawToken string) error
}

// SessionRegistry binds UDP remote addresses to players and tracks
// the race each player currently belongs to.
type SessionRegistry struct {
	mu       sync.RWMutex
	addr     map[string]uuid.UUID    // remote.String() -> player
	player   map[uuid.UUID]addrEntry // player -> {addr,race}
}

// addrEntry holds the player's current remote address and race.
type addrEntry struct {
	key  string
	race *race.Race
}

// NewSessionRegistry constructs an empty registry.
func NewSessionRegistry() *SessionRegistry {
	return &SessionRegistry{
		addr:   map[string]uuid.UUID{},
		player: map[uuid.UUID]addrEntry{},
	}
}

// BindAddr remembers which UDP address belongs to which player.
// Returns false if the address is already bound to a different player.
func (r *SessionRegistry) BindAddr(addr *net.UDPAddr, playerID uuid.UUID) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := addr.String()
	if _, ok := r.addr[key]; ok {
		return false
	}
	r.addr[key] = playerID
	if _, ok := r.player[playerID]; !ok {
		r.player[playerID] = addrEntry{key: key}
	} else {
		e := r.player[playerID]
		e.key = key
		r.player[playerID] = e
	}
	return true
}

// SetRace records which race the player is in.
func (r *SessionRegistry) SetRace(playerID uuid.UUID, rc *race.Race) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.player[playerID]
	if !ok {
		return
	}
	e.race = rc
	r.player[playerID] = e
}

// PlayerFromAddr returns the player at the given address.
func (r *SessionRegistry) PlayerFromAddr(addr *net.UDPAddr) (uuid.UUID, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.addr[addr.String()]
	return p, ok
}

// RaceFromPlayer returns the race the player is in.
func (r *SessionRegistry) RaceFromPlayer(playerID uuid.UUID) (*race.Race, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.player[playerID]
	if !ok || e.race == nil {
		return nil, false
	}
	return e.race, true
}

// PlayerView is a (player_id, addr) tuple.
type PlayerView struct {
	PlayerID uuid.UUID
	Addr     *net.UDPAddr
}

// RacePlayers returns the players currently registered in `r`.
func (r *SessionRegistry) RacePlayers(rc *race.Race) []PlayerView {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []PlayerView
	for pid, e := range r.player {
		if e.race == rc {
			uaddr := &net.UDPAddr{}
			if e.key != "" {
				if host, port, err := net.SplitHostPort(e.key); err == nil {
					uaddr = &net.UDPAddr{IP: net.ParseIP(host), Port: parsePort(port)}
				}
			}
			out = append(out, PlayerView{PlayerID: pid, Addr: uaddr})
		}
	}
	return out
}

func parsePort(p string) int {
	var n int
	_, _ = fmt.Sscanf(p, "%d", &n)
	return n
}

// Dispatcher routes decoded Protobuf messages to handlers.
type Dispatcher struct {
	verifier TokenVerifier
	races    RaceManager
	sessions *SessionRegistry
	log      *slog.Logger
}

// NewDispatcher wires a dispatcher.
func NewDispatcher(v TokenVerifier, rm RaceManager, sess *SessionRegistry, log *slog.Logger) *Dispatcher {
	if log == nil {
		log = slog.Default()
	}
	return &Dispatcher{
		verifier: v,
		races:    rm,
		sessions: sess,
		log:      log,
	}
}

// Handle turns a raw payload into a reply payload.
func (d *Dispatcher) Handle(ctx context.Context, payload []byte, from *net.UDPAddr) []byte {
	// Optimization: try PlayerInput first if the message ends in any
	// of the well-known small field markers (heuristic). The simpler
	// / clearer implementation tries JoinRaceRequest first; the
	// PlayerInput path will be tried once dispatch sees it has no
	// match_id field set.
	req := &gamepb.JoinRaceRequest{}
	if err := proto.Unmarshal(payload, req); err == nil && req.GetMatchId() != "" && req.GetGameToken() != "" {
		return d.handleJoin(ctx, req, from)
	}
	in := &gamepb.PlayerInput{}
	if err := proto.Unmarshal(payload, in); err == nil && (in.GetThrottle() != 0 || in.GetBrake() != 0 || in.GetSteering() != 0) {
		return d.handleInput(in, from)
	}
	d.log.Debug("dispatcher.unknown",
		slog.String("from", from.String()),
		slog.Int("bytes", len(payload)),
	)
	return nil
}

func (d *Dispatcher) handleJoin(ctx context.Context, req *gamepb.JoinRaceRequest, from *net.UDPAddr) []byte {
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
	if err := d.verifier.VerifyAndConsume(ctx, matchID, playerID, rawToken); err != nil {
		d.log.Info("dispatcher.join.rejected",
			slog.String("match_id", matchID.String()),
			slog.String("player_id", playerID.String()),
			slog.String("err", err.Error()),
			slog.String("from", from.String()),
		)
		return marshalJoinResp(false, err.Error(), "", 0)
	}
	r, err := d.races.RegisterPlayer(ctx, matchID, playerID)
	if err != nil {
		return marshalJoinResp(false, err.Error(), "", 0)
	}
	d.sessions.BindAddr(from, playerID)
	d.sessions.SetRace(playerID, r)
	initialTick := uint64(time.Now().UnixMilli()) & 0xFFFFFFFF
	d.log.Info("dispatcher.join.ok",
		slog.String("match_id", matchID.String()),
		slog.String("player_id", playerID.String()),
		slog.String("race_id", r.ID.String()),
		slog.String("from", from.String()),
	)
	return marshalJoinResp(true, "", r.ID.String(), initialTick)
}

func (d *Dispatcher) handleInput(in *gamepb.PlayerInput, from *net.UDPAddr) []byte {
	playerID, ok := d.sessions.PlayerFromAddr(from)
	if !ok {
		return nil
	}
	r, ok := d.sessions.RaceFromPlayer(playerID)
	if !ok || r == nil {
		return nil
	}
	if r.Status() == race.StatusFinished {
		return nil
	}
	r.ApplyInput(race.InputMsg{
		PlayerID: playerID,
		Input: race.PlayerInput{
			Throttle: in.GetThrottle(),
			Brake:    in.GetBrake(),
			Steering: in.GetSteering(),
		},
		Sequence: in.GetSequence(),
	})
	return nil
}

func marshalJoinResp(ok bool, reason, raceID string, tick uint64) []byte {
	r := &gamepb.JoinRaceResponse{Ok: ok, Error: reason, RaceId: raceID, InitialTick: tick}
	out, err := proto.Marshal(r)
	if err != nil {
		return []byte{0x08, 0x00}
	}
	return out
}

// IsRetryable reports whether an error from the verifier (or any
// other caller) can be sent back to the Unity client as-is.
func IsRetryable(err error) bool {
	return err == nil ||
		err == matchmaking.ErrTokenInvalid ||
		err == matchmaking.ErrTokenExpired ||
		err == matchmaking.ErrTokenConsumed
}
