package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestIssueAndParseAccessToken_RoundTrip(t *testing.T) {
	secret := "test-secret-do-not-use"
	issuer := "test-issuer"
	playerID := uuid.New()
	userID := uuid.New()

	tok, err := IssueAccessToken(secret, issuer, time.Minute, userID, playerID)
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}
	if tok == "" {
		t.Fatal("expected non-empty token")
	}

	claims, err := ParseAccessToken(secret, issuer, tok)
	if err != nil {
		t.Fatalf("ParseAccessToken: %v", err)
	}
	if claims.PlayerID != playerID {
		t.Errorf("PlayerID = %s, want %s", claims.PlayerID, playerID)
	}
	if claims.UserID != userID {
		t.Errorf("UserID = %s, want %s", claims.UserID, userID)
	}
	if claims.Issuer != issuer {
		t.Errorf("Issuer = %q, want %q", claims.Issuer, issuer)
	}
}

func TestParseAccessToken_ExpiredRejected(t *testing.T) {
	secret := "x"
	issuer := "iss"
	playerID := uuid.New()
	userID := uuid.New()

	tok, err := IssueAccessToken(secret, issuer, -time.Minute, userID, playerID)
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}
	if _, err := ParseAccessToken(secret, issuer, tok); err != ErrUnauthorized {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestParseAccessToken_WrongSecretRejected(t *testing.T) {
	tok, err := IssueAccessToken("right", "iss", time.Minute, uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}
	if _, err := ParseAccessToken("wrong", "iss", tok); err != ErrUnauthorized {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestParseAccessToken_WrongIssuerRejected(t *testing.T) {
	tok, err := IssueAccessToken("s", "good-issuer", time.Minute, uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}
	if _, err := ParseAccessToken("s", "bad-issuer", tok); err != ErrUnauthorized {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestParseAccessToken_EmptyRejected(t *testing.T) {
	if _, err := ParseAccessToken("s", "iss", ""); err != ErrUnauthorized {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}