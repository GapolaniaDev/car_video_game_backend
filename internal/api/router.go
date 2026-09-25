// Package api builds the HTTP router for the REST API.
package api

import (
	"log/slog"
	"net/http"

	"github.com/gustavo/racing-game-backend/internal/api/handlers"
	"github.com/gustavo/racing-game-backend/internal/api/middleware"
	"github.com/gustavo/racing-game-backend/internal/auth"
	"github.com/gustavo/racing-game-backend/internal/cars"
	"github.com/gustavo/racing-game-backend/internal/database"
	"github.com/gustavo/racing-game-backend/internal/garage"
	"github.com/gustavo/racing-game-backend/internal/matchmaking"
	"github.com/gustavo/racing-game-backend/internal/player"
	"github.com/gustavo/racing-game-backend/internal/tracks"
)

// RouterDeps carries dependencies needed by route handlers.
type RouterDeps struct {
	DB          *database.Pool
	Log         *slog.Logger
	AuthService *auth.Service
	Matchmaking *matchmaking.Service // optional; nil keeps stub fallback
	JWTSecret   string               // required when AuthService is set; used to build the auth middleware
	JWTIssuer   string               // expected JWT issuer claim
}

// NewRouter returns a fully-configured *http.ServeMux exposing the
// /api/v1 surface.
func NewRouter(deps RouterDeps) http.Handler {
	mux := http.NewServeMux()

	// ─── Public endpoints ─────────────────────────────────────────
	healthDeps := handlers.HealthDeps{DB: deps.DB, Log: deps.Log}
	mux.HandleFunc("GET /api/v1/health", handlers.Health(healthDeps))

	if deps.AuthService != nil {
		ah := auth.NewHandlers(deps.AuthService, deps.Log)
		mux.HandleFunc("POST /api/v1/auth/register", ah.Register)
		mux.HandleFunc("POST /api/v1/auth/login", ah.Login)
		mux.HandleFunc("POST /api/v1/auth/guest", ah.Guest)
	} else {
		mux.HandleFunc("POST /api/v1/auth/register", handlers.Stub("/api/v1/auth/register"))
		mux.HandleFunc("POST /api/v1/auth/login", handlers.Stub("/api/v1/auth/login"))
		mux.HandleFunc("POST /api/v1/auth/guest", handlers.Stub("/api/v1/auth/guest"))
	}

	// ─── Protected endpoints ───────────────────────────────────────
	// Gated by JWT middleware — every handler below expects the
	// player id to be present in the request context.
	if deps.DB != nil && deps.JWTSecret != "" {
		mw := auth.Middleware(deps.JWTSecret, deps.JWTIssuer, deps.Log)

		// /players/me
		ph := player.NewHandler(player.NewRepo(deps.DB.Pool), deps.Log)
		mux.Handle("GET /api/v1/players/me", mw(http.HandlerFunc(ph.Me)))

		// /cars, /cars/{id}
		ch := cars.NewHandler(cars.NewRepo(deps.DB.Pool), deps.Log)
		mux.Handle("GET /api/v1/cars", mw(http.HandlerFunc(ch.List)))
		mux.Handle("GET /api/v1/cars/{id}", mw(http.HandlerFunc(ch.Get)))

		// /garage
		gh := garage.NewHandler(garage.NewRepo(deps.DB.Pool), deps.Log)
		mux.Handle("GET /api/v1/garage", mw(http.HandlerFunc(gh.List)))

		// /tracks, /tracks/{id}
		th := tracks.NewHandler(tracks.NewRepo(deps.DB.Pool), deps.Log)
		mux.Handle("GET /api/v1/tracks", mw(http.HandlerFunc(th.List)))
		mux.Handle("GET /api/v1/tracks/{id}", mw(http.HandlerFunc(th.Get)))
	}

	// /matchmaking/join
	if deps.Matchmaking != nil && deps.JWTSecret != "" {
		mw := auth.Middleware(deps.JWTSecret, deps.JWTIssuer, deps.Log)
		mh := matchmaking.NewHandlers(deps.Matchmaking, deps.Log)
		mux.Handle("POST /api/v1/matchmaking/join", mw(http.HandlerFunc(mh.Join)))
	}

	// ─── Future endpoint stubs (kept as 501 placeholders) ─────────
	stubPaths := []string{
		"/api/v1/players",
		"/api/v1/matchmaking",
		"/api/v1/races",
		"/api/v1/leaderboard",
	}
	for _, p := range stubPaths {
		mux.HandleFunc("GET "+p, handlers.Stub(p))
		mux.HandleFunc("POST "+p, handlers.Stub(p))
	}

	return middleware.Chain(mux, middleware.RequestLogger(deps.Log))
}