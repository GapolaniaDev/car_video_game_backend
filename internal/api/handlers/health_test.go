package handlers

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHealthWithoutDB verifies the basic 200 + JSON shape.
func TestHealthWithoutDB(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()

	h := Health(HealthDeps{Log: slog.Default()})
	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	body, _ := io.ReadAll(rec.Body)
	var got HealthResponse
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("invalid JSON body %q: %v", body, err)
	}
	if got.Status != "ok" {
		t.Errorf("Status = %q, want ok", got.Status)
	}
	if got.Service != "backend-api" {
		t.Errorf("Service = %q, want backend-api", got.Service)
	}
	if got.Database != "skipped" {
		t.Errorf("Database = %q, want skipped (DB nil)", got.Database)
	}
}

// TestStubReturnsNotImplemented ensures future endpoint stubs return 501.
func TestStubReturnsNotImplemented(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth", nil)
	rec := httptest.NewRecorder()

	h := Stub("/api/v1/auth")
	h(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501", rec.Code)
	}

	body, _ := io.ReadAll(rec.Body)
	var got NotImplementedJSON
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	if got.Error != "not implemented" {
		t.Errorf("Error = %q", got.Error)
	}
	if got.Path != "/api/v1/auth" {
		t.Errorf("Path = %q, want /api/v1/auth", got.Path)
	}
}