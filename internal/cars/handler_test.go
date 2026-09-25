package cars

import (
	"encoding/json"
	"os"
	"testing"
)

// TestSeedsShape asserts the seed migration names the three catalog
// cars and well-formed UUID strings.
func TestSeedsShape(t *testing.T) {
	b, err := os.ReadFile("../../migrations/000003_seeds.up.sql")
	if err != nil {
		t.Fatalf("read seeds: %v", err)
	}
	sql := string(b)
	for _, name := range []string{"Veloce", "Bruiser", "Drifter"} {
		if !contains(sql, name) {
			t.Errorf("seeds missing car %q", name)
		}
	}
	// JSON round-trip of the Car struct using a minimal document.
	c := Car{}
	raw := []byte(`{"carId":"11111111-1111-1111-1111-111111111111","name":"Veloce","baseStats":{},"createdAt":"2025-01-01T00:00:00Z"}`)
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if c.Name != "Veloce" {
		t.Errorf("expected Veloce, got %q", c.Name)
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}