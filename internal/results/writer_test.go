package results

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// TestRequestValidationFailsFast sanity-checks that a zero track or
// empty results surfaces a sentinel error before any DB call.
func TestRequestValidationFailsFast(t *testing.T) {
	w := &Writer{} // no DB; every call must reject input first.
	ctx := context.Background()
	cases := []struct {
		name string
		req  WriteRequest
	}{
		{"empty race id", WriteRequest{TrackID: uuid.New(), Results: []Entry{{PlayerID: uuid.New()}}}},
		{"empty track id", WriteRequest{RaceID: uuid.New(), Results: []Entry{{PlayerID: uuid.New()}}}},
		{"empty results", WriteRequest{RaceID: uuid.New(), TrackID: uuid.New()}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := w.PersistRaceFinish(ctx, tc.req)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
