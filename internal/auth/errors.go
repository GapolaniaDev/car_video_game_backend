// Package auth provides registration, login, guest account creation,
// JWT issuance, and JWT-validation middleware for the REST API.
//
// This package owns the user/player identity layer. It does NOT own
// matchmaking tokens — those live in internal/matchmaking.
package auth

import "errors"

// Sentinels surfaced to HTTP handlers so they can map to status codes.
var (
	// ErrInvalidCredentials is returned for any login failure (wrong
	// email, wrong password, account locked, …). Handlers MUST return
	// the same opaque message regardless of the underlying reason to
	// prevent user enumeration.
	ErrInvalidCredentials = errors.New("invalid credentials")

	// ErrEmailTaken is returned by Register when the email already
	// exists. Maps to HTTP 409.
	ErrEmailTaken = errors.New("email already in use")

	// ErrPlayerNotFound is returned when a player id resolves to no
	// row. Maps to HTTP 404.
	ErrPlayerNotFound = errors.New("player not found")

	// ErrInvalidInput is returned for malformed bodies or failing
	// validation rules. The wrapped error explains why.
	ErrInvalidInput = errors.New("invalid input")

	// ErrUnauthorized is the canonical "no/bad token" error from the
	// middleware. Handlers don't see this — middleware short-circuits.
	ErrUnauthorized = errors.New("unauthorized")
)