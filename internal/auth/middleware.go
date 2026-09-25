package auth

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// contextKey is unexported to avoid collisions with other packages'
// context keys.
type contextKey int

const (
	// CtxClaimsKey is the request context key for the parsed *Claims.
	CtxClaimsKey contextKey = iota
	// CtxPlayerIDKey is the request context key for the authenticated
	// player's UUID.
	CtxPlayerIDKey
)

// Middleware validates the Authorization header and attaches the
// parsed claims to the request context. On failure it short-circuits
// with 401 + a generic JSON body.
func Middleware(secret, issuer string, log *slog.Logger) func(http.Handler) http.Handler {
	if log == nil {
		log = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := extractBearer(r.Header.Get("Authorization"))
			if raw == "" {
				respondUnauthorized(w, log)
				return
			}
			claims, err := ParseAccessToken(secret, issuer, raw)
			if err != nil {
				respondUnauthorized(w, log)
				return
			}
			ctx := context.WithValue(r.Context(), CtxClaimsKey, claims)
			ctx = context.WithValue(ctx, CtxPlayerIDKey, claims.PlayerID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ClaimsFromContext returns the claims attached by Middleware, or nil
// if the request did not pass through it.
func ClaimsFromContext(ctx context.Context) *Claims {
	c, _ := ctx.Value(CtxClaimsKey).(*Claims)
	return c
}

// PlayerIDFromContext returns the authenticated player UUID and true
// when Middleware ran on this request.
func PlayerIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(CtxPlayerIDKey).(uuid.UUID)
	return id, ok && id != uuid.Nil
}

func extractBearer(h string) string {
	const prefix = "Bearer "
	if len(h) < len(prefix) {
		return ""
	}
	if !strings.EqualFold(h[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(h[len(prefix):])
}

func respondUnauthorized(w http.ResponseWriter, log *slog.Logger) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", `Bearer realm="racing-game"`)
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
	log.Debug("auth.middleware.unauthorized")
}