package auth

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMiddleware_MissingHeaderReturns401(t *testing.T) {
	h := Middleware("s", "x", slog.Default())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["error"] != "unauthorized" {
		t.Errorf("body = %v", body)
	}
}

func TestMiddleware_ValidTokenAttachesClaims(t *testing.T) {
	secret := "s"
	issuer := "i"
	playerID := uuid.New()
	userID := uuid.New()
	tok, err := IssueAccessToken(secret, issuer, time.Minute, userID, playerID)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	var seen uuid.UUID
	h := Middleware(secret, issuer, slog.Default())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := PlayerIDFromContext(r.Context())
		if !ok {
			t.Fatal("PlayerIDFromContext returned false")
		}
		seen = id
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	}))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if seen != playerID {
		t.Errorf("ctx player id = %s, want %s", seen, playerID)
	}
}

func TestMiddleware_InvalidTokenReturns401(t *testing.T) {
	h := Middleware("s", "i", slog.Default())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer not-a-jwt")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestMiddleware_WrongSchemeReturns401(t *testing.T) {
	h := Middleware("s", "i", slog.Default())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Basic foo")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}