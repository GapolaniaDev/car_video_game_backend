// Package main starts the UDP Game Server.
//
// Spec 19: the server decodes length-prefixed Protobuf, validates the
// game_token against the local Postgres `matchmaking_assignments` row
// and replies with a JoinRaceResponse. Future specs (race loop, input,
// snapshots) extend the dispatcher with player+session state.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/gustavo/racing-game-backend/internal/config"
	"github.com/gustavo/racing-game-backend/internal/database"
	"github.com/gustavo/racing-game-backend/internal/matchmaking"
	"github.com/gustavo/racing-game-backend/internal/networking"
)

func main() {
	cfg := config.MustLoad()

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(log)

	log.Info("starting game-server",
		slog.String("env", cfg.AppEnv),
		slog.Int("udp_port", cfg.GameServerPort),
	)

	// Establish the DB pool the dispatcher needs for token verification.
	startupCtx, startupCancel := context.WithTimeout(context.Background(), cfg.PostgresReadyTimeout())
	defer startupCancel()

	pool, err := database.New(startupCtx, cfg, log)
	if err != nil {
		log.Error("postgres not reachable from game-server", slog.String("error", err.Error()))
		os.Exit(1)
	}
	matchRepo := matchmaking.NewRepo(pool.Pool)
	matchRepo.SetTokenSecret(cfg.GameTokenSecret)
	defer pool.Close()

	// Wire UDP + dispatcher.
	srv, err := networking.NewUDPServer(cfg.GameServerPort, log)
	if err != nil {
		log.Error("failed to bind udp socket", slog.String("error", err.Error()))
		os.Exit(1)
	}
	srv.SetDispatcher(networking.NewDispatcher(matchRepo, nil, log))
	log.Info("udp server listening",
		slog.String("addr", srv.LocalAddr().String()),
		slog.String("public_host", cfg.GameServerPublicHost),
		slog.Int("public_port", cfg.GameServerPublicPort),
	)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := srv.Serve(ctx); err != nil {
		log.Error("udp server error", slog.String("error", err.Error()))
	}

	log.Info("closing udp socket")
	if err := srv.Close(); err != nil {
		log.Warn("udp close error", slog.String("error", err.Error()))
	}

	fmt.Println("game-server stopped")
}