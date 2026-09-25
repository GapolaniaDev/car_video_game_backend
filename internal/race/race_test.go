package race

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func newSquareTrack() *Track {
	return &Track{
		ID:   uuid.New(),
		Name: "Square",
		Waypoints: []Waypoint{
			{X: 0, Z: 0}, {X: 100, Z: 0}, {X: 100, Z: 100}, {X: 0, Z: 100},
		},
		Spawns: []Vec3{{X: 0, Z: 0}, {X: 0, Z: 10}, {X: 0, Z: 20}, {X: 0, Z: 30}},
	}
}

func TestStepMovesForwardUnderThrottle(t *testing.T) {
	st := &PlayerState{}
	Step(st, PlayerInput{Throttle: 1}, DefaultSpec(), 1.0/30)
	if st.Velocity <= 0 {
		t.Fatalf("expected positive velocity, got %v", st.Velocity)
	}
	if st.Position.X == 0 && st.Position.Z == 0 {
		t.Fatalf("expected position to advance, got %+v", st.Position)
	}
}

func TestStepClampsSpeed(t *testing.T) {
	st := &PlayerState{}
	for i := 0; i < 1000; i++ {
		Step(st, PlayerInput{Throttle: 1}, DefaultSpec(), 1.0/30)
	}
	if st.Velocity > DefaultSpec().MaxSpeed+0.01 {
		t.Fatalf("speed not clamped: %v > %v", st.Velocity, DefaultSpec().MaxSpeed)
	}
}

func TestNewTrackLen(t *testing.T) {
	tr := newSquareTrack()
	if tr.Len() != 4 {
		t.Fatalf("got %d want 4", tr.Len())
	}
	if tr.NextCheckpoint(3) != 0 {
		t.Fatalf("wrap-around wrong: %d", tr.NextCheckpoint(3))
	}
}

func TestRaceRunOnePlayerCompletes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Tiny loop: 10m square. Player at (0,0) needs 4 waypoints at
	// (5,0),(5,5),(0,5),(0,0). With Mf. accel=12 and MaxSpeed=30 the
	// race completes well within the test timeout.
	tr := &Track{
		ID:   uuid.New(),
		Name: "Loop10",
		Waypoints: []Waypoint{
			{X: 0, Z: 0}, {X: 5, Z: 0}, {X: 5, Z: 5}, {X: 0, Z: 5},
		},
	}
	r := New(uuid.New(), tr, 60, 1, 4, nil)
	playerID := uuid.New()
	r.AddPlayer(playerID, DefaultSpec())

	go r.Run(ctx)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case r.InputCh <- InputMsg{
				PlayerID: playerID,
				Input:    PlayerInput{Throttle: 1},
				Sequence: 1,
			}:
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()

	deadline := time.After(15 * time.Second)
	for {
		select {
		case <-deadline:
			snap := r.PlayersSnapshot()
			extra := ""
			if len(snap) > 0 {
				extra = " lap=" + itoa(snap[0].Lap) + " pos=" + sprintVec(snap[0].Position)
			}
			t.Fatalf("race did not finish; status=%s players=%d%s",
				r.Status(), len(snap), extra)
		default:
		}
		if r.Status() == StatusFinished {
			results := r.FinalResults()
			if len(results) != 1 {
				t.Fatalf("expected 1 result, got %d (%+v)", len(results), results)
			}
			if results[0].PlayerID != playerID {
				t.Fatalf("expected player %s, got %s", playerID, results[0].PlayerID)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	out := ""
	for i > 0 {
		out = string(rune('0'+i%10)) + out
		i /= 10
	}
	if neg {
		return "-" + out
	}
	return out
}

func sprintVec(v Vec3) string { return "{" + itoa(int(v.X)) + "," + itoa(int(v.Y)) + "," + itoa(int(v.Z)) + "}" }

func TestRaceManagerRegistersPlayer(t *testing.T) {
	m := NewManager(60, 1, 4, func(_ uuid.UUID) (*Track, error) {
		return newSquareTrack(), nil
	}, nil)
	r, err := m.RegisterPlayer(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("RegisterPlayer: %v", err)
	}
	if r == nil {
		t.Fatal("nil race")
	}
	if len(r.PlayersSnapshot()) != 1 {
		t.Fatalf("expected 1 player, got %d", len(r.PlayersSnapshot()))
	}
}
