// Package main starts the REST API server.
//
// Responsibilities:
//   * Load configuration and initialize structured logging (slog).
//   * Optionally open a Postgres connection pool (best-effort: API
//     still boots if DB is briefly unavailable; /health surfaces
//     state).
//   * Start the HTTP server and wait for SIGINT/SIGTERM, then run a
//     graceful shutdown sequence: stop accepting connections, finish
//     in-flight requests, close the DB pool.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gustavo/racing-game-backend/internal/api"
	"github.com/gustavo/racing-game-backend/internal/auth"
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

	// DB connection is best-effort: a missing Postgres must not stop
	// the API from booting. The /health endpoint reports state.
	startupCtx, startupCancel := context.WithTimeout(context.Background(), cfg.PostgresReadyTimeout())
	defer startupCancel()

	var pool *database.Pool
	var authSvc *auth.Service
	p, err := database.New(startupCtx, cfg, log)
	if err != nil {
		log.Warn("database not reachable at startup", slog.String("error", err.Error()))
	} else {
		pool = p
		authSvc = auth.NewService(
			auth.NewRepo(pool.Pool),
			cfg.JWTSecret,
			cfg.JWTIssuer,
			cfg.JWTAccessTTL,
			log,
		)
	}

	router := api.NewRouter(api.RouterDeps{
		DB:          pool,
		Log:         log,
		AuthService: authSvc,
	})

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.APIPort),
		Handler:           router,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serveErrCh := make(chan error, 1)
	go func() {
		log.Info("http server listening", slog.String("addr", server.Addr))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErrCh <- err
			return
		}
		serveErrCh <- nil
	}()

	select {
	case err := <-serveErrCh:
		if err != nil {
			log.Error("http server error", slog.String("error", err.Error()))
			cleanupAndExit(pool, log, 1)
			return
		}
	case <-rootCtx.Done():
		log.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("http server shutdown error", slog.String("error", err.Error()))
	} else {
		log.Info("http server stopped cleanly")
	}

	cleanupAndExit(pool, log, 0)
}

func cleanupAndExit(pool *database.Pool, log *slog.Logger, code int) {
	if pool != nil {
		pool.Close()
	}
	log.Info("backend-api stopped")
	os.Exit(code)
}