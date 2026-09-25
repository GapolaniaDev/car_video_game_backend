package matchmaking

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// TokenTTL is the lifetime of every game_token issued by the API.
// Short enough to bound replay risk, long enough to cover the Unity
// handshake from /matchmaking/join to the Game Server's Join.
const TokenTTL = 5 * time.Minute

// tokenPayload is the inner JSON document encoded in game_token.
type tokenPayload struct {
	MatchID  uuid.UUID `json:"match_id"`
	PlayerID uuid.UUID `json:"player_id"`
	Exp      int64     `json:"exp"` // unix seconds
}

// SignGameToken signs a game_token for the given (match, player) pair
// using HMAC-SHA256 over base64url(payload). The returned string is
// `payload.sig` in base64url. The verifier must use the same secret.
func SignGameToken(secret string, matchID, playerID uuid.UUID, now time.Time) (string, time.Time, error) {
	if secret == "" {
		return "", time.Time{}, errors.New("matchmaking: empty secret")
	}
	if matchID == uuid.Nil || playerID == uuid.Nil {
		return "", time.Time{}, errors.New("matchmaking: zero uuid")
	}
	exp := now.Add(TokenTTL).Unix()
	raw, err := json.Marshal(tokenPayload{MatchID: matchID, PlayerID: playerID, Exp: exp})
	if err != nil {
		return "", time.Time{}, err
	}
	payload := base64.RawURLEncoding.EncodeToString(raw)
	sig := base64.RawURLEncoding.EncodeToString(hmacSHA256([]byte(payload), secret))
	return payload + "." + sig, time.Unix(exp, 0), nil
}

// VerifyGameToken validates a token's signature, expiration, and (when
// db is non-nil) existence/non-consumption in matchmaking_assignments.
// When db is nil only the HMAC + exp are checked (useful in tests).
func VerifyGameToken(secret, raw string, db TokenStore, now time.Time) (*tokenPayload, error) {
	parts := splitToken(raw)
	if len(parts) != 2 {
		return nil, ErrTokenInvalid
	}
	expected := base64.RawURLEncoding.EncodeToString(hmacSHA256([]byte(parts[0]), secret))
	if !hmac.Equal([]byte(expected), []byte(parts[1])) {
		return nil, ErrTokenInvalid
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrTokenInvalid
	}
	var pl tokenPayload
	if err := json.Unmarshal(body, &pl); err != nil {
		return nil, ErrTokenInvalid
	}
	if now.Unix() > pl.Exp {
		return nil, ErrTokenExpired
	}
	if db != nil {
		ok, err := db.AssignmentIsFresh(pl.MatchID, pl.PlayerID, raw, now)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrTokenInvalid
		}
	}
	return &pl, nil
}

// TokenStore is the DB-backed checker used by VerifyGameToken when
// realtime replay protection is required.
type TokenStore interface {
	// AssignmentIsFresh returns true when a row with the given
	// (matchID, playerID, game_token) exists, is unconsumed and not
	// expired. Any check failure (including not-found) returns
	// (false, nil).
	AssignmentIsFresh(matchID, playerID uuid.UUID, rawToken string, now time.Time) (bool, error)

	// MarkConsumed flips the consumed flag for the row identified by
	// (matchID, playerID, game_token).
	MarkConsumed(matchID, playerID uuid.UUID, rawToken string) error
}

// splitToken splits on the LAST '.' so the payload (which is
// itself base64url with no '.') stays intact. Both halves are the
// base64url alphabet.
func splitToken(s string) []string {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return nil
}

// VerifyHMAC is a single-purpose helper that only checks the HMAC
// signature on a token. It does NOT validate the JSON payload or
// expiry; use VerifyGameToken for the full check.
func VerifyHMAC(secret, raw string) bool {
	parts := splitToken(raw)
	if len(parts) != 2 {
		return false
	}
	expected := base64.RawURLEncoding.EncodeToString(hmacSHA256([]byte(parts[0]), secret))
	return hmac.Equal([]byte(expected), []byte(parts[1]))
}

func hmacSHA256(msg []byte, secret string) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(msg)
	return mac.Sum(nil)
}

// String implements fmt.Stringer for log-safe payloads. It returns
// the match and player ids without the token itself.
func (p *tokenPayload) String() string { return fmt.Sprintf("match=%s player=%s", p.MatchID, p.PlayerID) }
