// Package networking — race broadcast helper.
//
// The broadcaster subscribes to a race's SnapshotOut channel and
// fans out UDP datagrams to every registered player in the race.
// Player UDP addresses are added on JoinRaceRequest success and
// dropped on disconnect.
package networking

import (
	"context"
	"log/slog"
	"net"
	"sync"

	"github.com/google/uuid"
	"github.com/gustavo/racing-game-backend/internal/race"
	"github.com/gustavo/racing-game-backend/protocol/gamepb"
	"google.golang.org/protobuf/proto"
)

// UDPWriter is the minimum contract the broadcaster needs from the
// underlying transport. *UDPServer satisfies it.
type UDPWriter interface {
	WriteToUDP(b []byte, addr *net.UDPAddr) (int, error)
}

// Subscription tracks one remote player's address inside a race.
type Subscription struct {
	PlayerID uuid.UUID
	Addr     *net.UDPAddr
}

// Subscriptions is the broadcaster's per-race lookup.
type Subscriptions struct {
	mu  sync.RWMutex
	by  map[uuid.UUID]*Subscription // player id
	all []*Subscription
}

// NewSubscriptions builds an empty subscription table.
func NewSubscriptions() *Subscriptions {
	return &Subscriptions{by: map[uuid.UUID]*Subscription{}}
}

// Add registers a player; returns false if already known.
func (s *Subscriptions) Add(playerID uuid.UUID, addr *net.UDPAddr) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.by[playerID]; ok {
		return false
	}
	sub := &Subscription{PlayerID: playerID, Addr: addr}
	s.by[playerID] = sub
	s.all = append(s.all, sub)
	return true
}

// Remove drops a player.
func (s *Subscriptions) Remove(playerID uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.by, playerID)
	filtered := s.all[:0]
	for _, sub := range s.all {
		if sub.PlayerID != playerID {
			filtered = append(filtered, sub)
		}
	}
	s.all = filtered
}

// All returns every active subscription.
func (s *Subscriptions) All() []*Subscription {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Subscription, len(s.all))
	copy(out, s.all)
	return out
}

// Broadcaster consumes snapshots from a single race and writes a
// Protobuf-encoded WorldSnapshot to every subscribed player.
type Broadcaster struct {
	race  *race.Race
	subs  *Subscriptions
	writer UDPWriter
	log    *slog.Logger
}

// NewBroadcaster wires a broadcaster.
func NewBroadcaster(r *race.Race, subs *Subscriptions, w UDPWriter, log *slog.Logger) *Broadcaster {
	if log == nil {
		log = slog.Default()
	}
	return &Broadcaster{race: r, subs: subs, writer: w, log: log}
}

// Run drains the race's SnapshotOut channel until ctx is canceled or
// the race closes Done.
func (b *Broadcaster) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-b.race.Done:
			return nil
		case snap, ok := <-b.race.SnapshotOut:
			if !ok {
				return nil
			}
			b.publish(snap)
		}
	}
}

func (b *Broadcaster) publish(snap race.WorldSnapshot) {
	pb := &gamepb.WorldSnapshot{Tick: snap.Tick}
	for _, c := range snap.Cars {
		pb.Cars = append(pb.Cars, &gamepb.CarState{
			PlayerId: c.PlayerID,
			Position: &c.Position,
			Rotation: &c.Rotation,
			Velocity: &c.Velocity,
		})
	}
	payload, err := proto.Marshal(pb)
	if err != nil {
		b.log.Error("broadcast.marshal", slog.String("err", err.Error()))
		return
	}
	// Length-prefix the payload before writing.
	framed, err := framingEncode(payload)
	if err != nil {
		b.log.Error("broadcast.frame", slog.String("err", err.Error()))
		return
	}
	for _, sub := range b.subs.All() {
		if sub.Addr == nil {
			continue
		}
		if _, err := b.writer.WriteToUDP(framed, sub.Addr); err != nil {
			b.log.Warn("broadcast.write",
				slog.String("player_id", sub.PlayerID.String()),
				slog.String("addr", sub.Addr.String()),
				slog.String("err", err.Error()),
			)
		}
	}
}

// framingEncode is inlined from protoframing to avoid importing the
// package from inside the dispatch path (small dependency budget
// when the Game Server binary links everything).
func framingEncode(payload []byte) ([]byte, error) {
	const hdr = 4
	if len(payload) > 64*1024 {
		return nil, errFrameTooBig
	}
	out := make([]byte, hdr+len(payload))
	out[0] = byte(len(payload) >> 24)
	out[1] = byte(len(payload) >> 16)
	out[2] = byte(len(payload) >> 8)
	out[3] = byte(len(payload))
	copy(out[hdr:], payload)
	return out, nil
}

var errFrameTooBig = errTooBig{}

type errTooBig struct{}

func (errTooBig) Error() string { return "broadcast: frame too big" }
