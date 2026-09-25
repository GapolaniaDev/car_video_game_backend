package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRequestLoggerEmitsStructuredEntry ensures the logger writes a
// single JSON record per request with the expected fields.
func TestRequestLoggerEmitsStructuredEntry(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("hello"))
	})

	h := RequestLogger(log)(inner)
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	if len(lines) != 1 {
		t.Fatalf("expected 1 log line, got %d: %s", len(lines), buf.String())
	}
	var entry map[string]any
	if err := json.Unmarshal(lines[0], &entry); err != nil {
		t.Fatalf("decode log: %v", err)
	}
	if entry["method"] != "GET" {
		t.Errorf("method=%v", entry["method"])
	}
	if entry["path"] != "/x" {
		t.Errorf("path=%v", entry["path"])
	}
	// JSON numbers decode to float64.
	if status, _ := entry["status"].(float64); status != http.StatusTeapot {
		t.Errorf("status=%v", entry["status"])
	}
}

// TestChainOrdering verifies middleware executes in declared order.
func TestChainOrdering(t *testing.T) {
	order := []string{}
	mw := func(name string) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name+":in")
				next.ServeHTTP(w, r)
				order = append(order, name+":out")
			})
		}
	}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "inner")
		w.WriteHeader(http.StatusOK)
	})

	h := Chain(inner, mw("a"), mw("b"))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	want := []string{"a:in", "b:in", "inner", "b:out", "a:out"}
	if len(order) != len(want) {
		t.Fatalf("order=%v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("order[%d]=%q, want %q", i, order[i], want[i])
		}
	}
	_ = io.Discard // keep io imported if future tests need it
}