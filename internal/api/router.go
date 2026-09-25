// Package api builds the HTTP router for the REST API.
package api

import (
	"log/slog"
	"net/http"

	"github.com/gustavo/racing-game-backend/internal/api/handlers"
	"github.com/gustavo/racing-game-backend/internal/api/middleware"
	"github.com/gustavo/racing-game-backend/internal/auth"
	"github.com/gustavo/racing-game-backend/internal/database"
)

// RouterDeps carries dependencies needed by route handlers.
type RouterDeps struct {
	DB          *database.Pool
	Log         *slog.Logger
	AuthService *auth.Service // optional; when set, real /auth handlers are wired
}

// NewRouter returns a fully-configured *http.ServeMux exposing the
// /api/v1 surface. Future endpoints are registered as 501 stubs.
func NewRouter(deps RouterDeps) http.Handler {
	mux := http.NewServeMux()

	healthDeps := handlers.HealthDeps{DB: deps.DB, Log: deps.Log}
	mux.HandleFunc("GET /api/v1/health", handlers.Health(healthDeps))

	// ─── /auth (real or stub) ────────────────────────────────────────
	if deps.AuthService != nil {
		ah := auth.NewHandlers(deps.AuthService, deps.Log)
		mux.HandleFunc("POST /api/v1/auth/register", ah.Register)
		mux.HandleFunc("POST /api/v1/auth/login", ah.Login)
		mux.HandleFunc("POST /api/v1/auth/guest", ah.Guest)
	} else {
		// Fallback (Spec 05 behaviour) so the router can be built
		// without a DB connection during smoke tests.
		mux.HandleFunc("POST /api/v1/auth/register", handlers.Stub("/api/v1/auth/register"))
		mux.HandleFunc("POST /api/v1/auth/login", handlers.Stub("/api/v1/auth/login"))
		mux.HandleFunc("POST /api/v1/auth/guest", handlers.Stub("/api/v1/auth/guest"))
	}

	// ─── Future endpoint stubs (always 501 today) ────────────────────
	stubPaths := []string{
		"/api/v1/players",
		"/api/v1/players/me",
		"/api/v1/cars",
		"/api/v1/garage",
		"/api/v1/tracks",
		"/api/v1/matchmaking",
		"/api/v1/matchmaking/join",
		"/api/v1/races",
		"/api/v1/leaderboard",
	}
	for _, p := range stubPaths {
		mux.HandleFunc("GET "+p, handlers.Stub(p))
		mux.HandleFunc("POST "+p, handlers.Stub(p))
	}

	return middleware.Chain(mux, middleware.RequestLogger(deps.Log))
}