package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gustavo/racing-game-backend/internal/api/handlers"
)

// TestRouterHealthAndStubs verifies that the router dispatches both
// real handlers and stub handlers correctly.
func TestRouterHealthAndStubs(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewRouter(RouterDeps{Log: log})

	tests := []struct {
		method     string
		path       string
		wantStatus int
	}{
		{http.MethodGet, "/api/v1/health", http.StatusOK},
		{http.MethodPost, "/api/v1/auth/register", http.StatusNotImplemented}, // stub: no AuthService in test
		{http.MethodPost, "/api/v1/auth/login", http.StatusNotImplemented},
		{http.MethodPost, "/api/v1/auth/guest", http.StatusNotImplemented},
		{http.MethodGet, "/api/v1/matchmaking", http.StatusNotImplemented},
		{http.MethodPost, "/api/v1/matchmaking/join", http.StatusNotImplemented},
		{http.MethodGet, "/api/v1/leaderboard", http.StatusNotImplemented},
		{http.MethodGet, "/api/v1/no-such-path", http.StatusNotImplemented},
	}
	// Last entry: unknown paths return 404 (default mux).
	tests[len(tests)-1].wantStatus = http.StatusNotFound

	for _, tc := range tests {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != tc.wantStatus {
			t.Errorf("%s %s: status=%d want=%d", tc.method, tc.path, rec.Code, tc.wantStatus)
		}
	}
}

// TestRouterHealthJSONShape double-checks the JSON body of the health endpoint.
func TestRouterHealthJSONShape(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewRouter(RouterDeps{Log: log})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rec.Code)
	}
	var body handlers.HealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Status != "ok" {
		t.Errorf("Status=%q, want ok", body.Status)
	}
	if body.Service != "backend-api" {
		t.Errorf("Service=%q, want backend-api", body.Service)
	}
}