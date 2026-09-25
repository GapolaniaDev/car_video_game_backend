// Package matchmaking mints short-lived game tokens and pairs players
// into live matches.
package matchmaking

import "errors"

// Sentinel errors that callers can match on.
var (
	// ErrNoGameServer indicates the API has no game-server routing
	// info to publish (misconfigured).
	ErrNoGameServer = errors.New("matchmaking: no game server configured")

	// ErrRaceFull indicates a pending match already has the max
	// number of players (4).
	ErrRaceFull = errors.New("matchmaking: race full")

	// ErrTokenInvalid indicates the HMAC failed, the payload is
	// malformed, or the assignment is unknown.
	ErrTokenInvalid = errors.New("matchmaking: invalid token")

	// ErrTokenExpired indicates the token's exp claim is in the past.
	ErrTokenExpired = errors.New("matchmaking: token expired")

	// ErrTokenConsumed indicates the token was already used by a prior
	// JoinRaceRequest.
	ErrTokenConsumed = errors.New("matchmaking: token already consumed")
)
