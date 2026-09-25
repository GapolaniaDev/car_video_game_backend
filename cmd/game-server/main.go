// Package main starts the UDP Game Server.
//
// Spec 08: binds a UDP socket on GAME_SERVER_PORT and replies to PING
// with PONG. Future milestones will replace HandlePacket with
// Protobuf decoding and add per-race goroutines.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/gustavo/racing-game-backend/internal/config"
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

	srv, err := networking.NewUDPServer(cfg.GameServerPort, log)
	if err != nil {
		log.Error("failed to bind udp socket", slog.String("error", err.Error()))
		os.Exit(1)
	}
	log.Info("udp server listening", slog.String("addr", srv.LocalAddr().String()))

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