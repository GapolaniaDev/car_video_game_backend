package tracks

import (
	"encoding/json"
	"os"
	"testing"
)

func TestSeedsShape(t *testing.T) {
	b, err := os.ReadFile("../../migrations/000003_seeds.up.sql")
	if err != nil {
		t.Fatalf("read seeds: %v", err)
	}
	sql := string(b)
	for _, name := range []string{"Crescent Bay", "Granite Pass"} {
		if !contains(sql, name) {
			t.Errorf("seeds missing track %q", name)
		}
	}
	tr := Track{}
	raw := []byte(`{"trackId":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa","name":"Crescent Bay","layout":{},"createdAt":"2025-01-01T00:00:00Z"}`)
	if err := json.Unmarshal(raw, &tr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if tr.Name != "Crescent Bay" {
		t.Errorf("expected Crescent Bay, got %q", tr.Name)
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