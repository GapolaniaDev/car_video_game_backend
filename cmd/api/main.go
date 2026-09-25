// Package main starts the REST API server.
//
// Spec 05: HTTP server, /api/v1/health endpoint, structured logging,
// and route stubs for future endpoints. Graceful shutdown and DB
// integration land in Spec 13.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/gustavo/racing-game-backend/internal/api"
	"github.com/gustavo/racing-game-backend/internal/config"
	"github.com/gustavo/racing-game-backend/internal/database"
)

func main() {
	cfg := config.MustLoad()

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(log)

	log.Info("starting backend-api",
		slog.String("env", cfg.AppEnv),
		slog.Int("port", cfg.APIPort),
	)

	// DB connection is best-effort here. The /health endpoint
	// surfaces DB state, so the API itself can boot and respond even
	// when Postgres is briefly unavailable.
	ctx, cancel := context.WithTimeout(context.Background(), cfg.PostgresReadyTimeout())
	defer cancel()

	var pool *database.Pool
	p, err := database.New(ctx, cfg, log)
	if err != nil {
		log.Warn("database not reachable at startup", slog.String("error", err.Error()))
	} else {
		pool = p
	}

	router := api.NewRouter(api.RouterDeps{DB: pool, Log: log})

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.APIPort),
		Handler:           router,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Spec 13 will replace this with proper signal handling. For
	// Spec 05 we just ListenAndServe and exit on error.
	log.Info("http server listening", slog.String("addr", server.Addr))
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("http server error", slog.String("error", err.Error()))
		os.Exit(1)
	}

	log.Info("backend-api stopped")
}