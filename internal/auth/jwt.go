package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims is the payload embedded in every issued access token. Only
// IDs travel in the token; no email, no display name (callers should
// re-fetch profile via /players/me using the token).
type Claims struct {
	UserID   uuid.UUID `json:"uid"`
	PlayerID uuid.UUID `json:"sub"`
	jwt.RegisteredClaims
}

// IssueAccessToken signs a new access token for the given player.
// The token carries `sub` = playerID, `uid` = userID, plus `iss`,
// `iat`, and `exp` (now+ttl).
func IssueAccessToken(secret, issuer string, ttl time.Duration, userID, playerID uuid.UUID) (string, error) {
	if secret == "" {
		return "", errors.New("auth: empty JWT secret")
	}
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		PlayerID: playerID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   playerID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString([]byte(secret))
}

// ParseAccessToken verifies the signature, expiry, and issuer of a
// raw JWT and returns the parsed claims. Errors are mapped to the
// package's sentinels where possible.
func ParseAccessToken(secret, issuer, raw string) (*Claims, error) {
	if raw == "" {
		return nil, ErrUnauthorized
	}
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	},
		jwt.WithIssuer(issuer),
		jwt.WithValidMethods([]string{"HS256"}),
	)
	if err != nil {
		return nil, ErrUnauthorized
	}
	if claims.PlayerID == uuid.Nil || claims.UserID == uuid.Nil {
		return nil, ErrUnauthorized
	}
	return claims, nil
}