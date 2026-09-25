package matchmaking

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

const testSecret = "test_secret_do_not_reuse"

func TestSignAndVerifyRoundTrip(t *testing.T) {
	now := time.Now()
	matchID := uuid.New()
	playerID := uuid.New()
	token, exp, err := SignGameToken(testSecret, matchID, playerID, now)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if token == "" {
		t.Fatal("empty token")
	}
	if !exp.After(now) {
		t.Fatalf("exp %v must be after now %v", exp, now)
	}
	pl, err := VerifyGameToken(testSecret, token, nil, now)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if pl.MatchID != matchID || pl.PlayerID != playerID {
		t.Fatalf("payload mismatch: %+v", pl)
	}
}

func TestVerifyRejectsTampered(t *testing.T) {
	matchID := uuid.New()
	playerID := uuid.New()
	tok, _, _ := SignGameToken(testSecret, matchID, playerID, time.Now())
	// Flip the last char of the signature.
	last := tok[len(tok)-1]
	flipped := tok[:len(tok)-1]
	if last == 'A' {
		flipped += "B"
	} else {
		flipped += "A"
	}
	_, err := VerifyGameToken(testSecret, flipped, nil, time.Now())
	if !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("expected ErrTokenInvalid, got %v", err)
	}
}

func TestVerifyRejectsWrongSecret(t *testing.T) {
	matchID := uuid.New()
	playerID := uuid.New()
	tok, _, _ := SignGameToken(testSecret, matchID, playerID, time.Now())
	_, err := VerifyGameToken("other_secret", tok, nil, time.Now())
	if !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("expected ErrTokenInvalid, got %v", err)
	}
}

func TestVerifyRejectsExpired(t *testing.T) {
	matchID := uuid.New()
	playerID := uuid.New()
	issuedAt := time.Now().Add(-10 * time.Minute)
	// Re-sign with a backdated exp via a custom path: forge payload
	// with exp in past.
	pl := tokenPayload{MatchID: matchID, PlayerID: playerID, Exp: issuedAt.Unix()}
	raw := mustEncode(t, pl)
	sig := signRaw(testSecret, raw)
	tok := raw + "." + sig
	_, err := VerifyGameToken(testSecret, tok, nil, time.Now())
	if !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("expected ErrTokenExpired, got %v", err)
	}
}

func TestSignRejectsEmptySecret(t *testing.T) {
	_, _, err := SignGameToken("", uuid.New(), uuid.New(), time.Now())
	if err == nil {
		t.Fatal("expected error for empty secret")
	}
}

func TestSplitTokenIgnoresDotsInPayload(t *testing.T) {
	// Real tokens never contain dots in the payload portion, but the
	// splitter must anchor on the LAST dot so any accidental dots in
	// the signature space don't break the split.
	in := "abc.def.ghi"
	got := splitToken(in)
	if len(got) != 2 || got[0] != "abc.def" || got[1] != "ghi" {
		t.Fatalf("splitToken(%q) = %v", in, got)
	}
}
