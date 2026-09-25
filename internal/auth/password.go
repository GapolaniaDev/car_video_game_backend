package auth

import "golang.org/x/crypto/bcrypt"

// bcryptCost is intentionally above the default (10) to slow down
// brute-force attempts. Each increment doubles CPU work.
const bcryptCost = 12

// HashPassword returns a bcrypt hash of the plain-text password.
// Plaintext must be 8–128 bytes; the caller is expected to have
// validated length already. Returns ErrInvalidInput when the hash
// itself cannot be produced.
func HashPassword(plain string) (string, error) {
	if plain == "" {
		return "", ErrInvalidInput
	}
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

// VerifyPassword compares a stored bcrypt hash to a candidate plain
// password. Returns nil on match, ErrInvalidCredentials otherwise
// (we deliberately collapse bcrypt's error into our own sentinel so
// callers don't accidentally log "hash mismatch").
func VerifyPassword(hash, plain string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)); err != nil {
		return ErrInvalidCredentials
	}
	return nil
}