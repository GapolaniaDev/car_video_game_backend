// Package api builds the HTTP router for the REST API.
package api

import (
	"log/slog"
	"net/http"

	"github.com/gustavo/racing-game-backend/internal/api/handlers"
	"github.com/gustavo/racing-game-backend/internal/api/middleware"
	"github.com/gustavo/racing-game-backend/internal/database"
)

// RouterDeps carries dependencies needed by route handlers.
type RouterDeps struct {
	DB  *database.Pool // may be nil
	Log *slog.Logger
}

// NewRouter returns a fully-configured *http.ServeMux exposing the
// /api/v1 surface. Future endpoints are registered as 501 stubs.
func NewRouter(deps RouterDeps) http.Handler {
	mux := http.NewServeMux()

	healthDeps := handlers.HealthDeps{DB: deps.DB, Log: deps.Log}
	mux.HandleFunc("GET /api/v1/health", handlers.Health(healthDeps))

	// Future endpoint stubs — return 501 Not Implemented today.
	stubPaths := []string{
		"/api/v1/auth",
		"/api/v1/auth/login",
		"/api/v1/auth/guest",
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

	// Apply logging middleware to every route. Chain returns the
	// composed handler ready to be wrapped by http.Server.
	return middleware.Chain(mux, middleware.RequestLogger(deps.Log))
}