package gamepb

import (
	"testing"

	"google.golang.org/protobuf/proto"
)

// TestPlayerInputRoundTrip ensures a simple message can be serialized
// and deserialized without loss. Acts as a smoke test that the
// generated code is wired into the build correctly.
func TestPlayerInputRoundTrip(t *testing.T) {
	in := &PlayerInput{Sequence: 42, Throttle: 0.75, Brake: 0.0, Steering: -0.25}
	data, err := proto.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	out := &PlayerInput{}
	if err := proto.Unmarshal(data, out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.Sequence != in.Sequence || out.Throttle != in.Throttle ||
		out.Brake != in.Brake || out.Steering != in.Steering {
		t.Errorf("round-trip mismatch: in=%+v out=%+v", in, out)
	}
}

// TestWorldSnapshotCars confirms a nested repeated field serializes.
func TestWorldSnapshotCars(t *testing.T) {
	snap := &WorldSnapshot{
		Tick: 1,
		Cars: []*CarState{
			{PlayerId: "p1", Position: &Vector3{X: 1, Y: 2, Z: 3}},
		},
	}
	data, err := proto.Marshal(snap)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if len(data) < 4 {
		t.Errorf("serialized payload suspiciously small: %d bytes", len(data))
	}
}