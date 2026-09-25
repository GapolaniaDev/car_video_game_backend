// Package main starts the UDP Game Server.
//
// Specs 19 + 20: validates the game_token against Postgres, dispatches
// the JoinRaceRequest to the per-race state machine, accepts
// PlayerInput, and broadcasts WorldSnapshot back to every player in
// the race.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gustavo/racing-game-backend/internal/config"
	"github.com/gustavo/racing-game-backend/internal/database"
	"github.com/gustavo/racing-game-backend/internal/matchmaking"
	"github.com/gustavo/racing-game-backend/internal/networking"
	"github.com/gustavo/racing-game-backend/internal/race"
	"github.com/gustavo/racing-game-backend/internal/results"
)

func main() {
	cfg := config.MustLoad()

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(log)

	log.Info("starting game-server",
		slog.String("env", cfg.AppEnv),
		slog.Int("udp_port", cfg.GameServerPort),
	)

	startupCtx, startupCancel := context.WithTimeout(context.Background(), cfg.PostgresReadyTimeout())
	pool, err := database.New(startupCtx, cfg, log)
	startupCancel()
	if err != nil {
		log.Error("postgres not reachable from game-server", slog.String("error", err.Error()))
		os.Exit(1)
	}
	matchRepo := matchmaking.NewRepo(pool.Pool)
	matchRepo.SetTokenSecret(cfg.GameTokenSecret)
	defer pool.Close()

	srv, err := networking.NewUDPServer(cfg.GameServerPort, log)
	if err != nil {
		log.Error("failed to bind udp socket", slog.String("error", err.Error()))
		os.Exit(1)
	}
	trackLookup := buildTrackLookup(pool.Pool)
	raceMgr := race.NewManager(30, 1, 4, trackLookup, nil, log)
	sessions := networking.NewSessionRegistry()
	srv.SetDispatcher(networking.NewDispatcher(matchRepo, raceMgr, sessions, log))
	log.Info("udp server listening",
		slog.String("addr", srv.LocalAddr().String()),
		slog.String("public_host", cfg.GameServerPublicHost),
		slog.Int("public_port", cfg.GameServerPublicPort),
	)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Results writer: persists race_results on StatusFinished. The
	// callback is wired into the manager here (after `ctx` exists) so
	// the persist call has a context to use for retries.
	resWriter := results.NewWriter(pool.Pool, log)
	persistRace := func(r *race.Race) {
		res := r.FinalResults()
		entries := make([]results.Entry, 0, len(res))
		for _, e := range res {
			entries = append(entries, results.Entry{
				PlayerID:    e.PlayerID,
				Position:    e.Position,
				TotalTimeMs: e.TotalTimeMs,
				BestLapMs:   e.BestLapMs,
			})
		}
		req := results.WriteRequest{
			RaceID:     r.ID,
			TrackID:    r.Track.ID,
			StartedAt:  r.StartRealTime,
			FinishedAt: r.FinishTime,
			Results:    entries,
		}
		resWriter.PersistWithRetry(ctx, req)
	}
	raceMgr.SetOnFinish(persistRace)

	// Match broadcaster pool: one Broadcaster per active race, each
	// holds the per-player subscription table wired to the player's
	// UDP address (filled on JoinRaceRequest).
	bc := newBroadcasterPool(raceMgr, sessions, srv, log)
	bc.wireNewRaces(ctx)
	go bc.watch(ctx)

	if err := srv.Serve(ctx); err != nil {
		log.Error("udp server error", slog.String("error", err.Error()))
	}

	log.Info("closing udp socket")
	if err := srv.Close(); err != nil {
		log.Warn("udp close error", slog.String("error", err.Error()))
	}
	fmt.Println("game-server stopped")
}

// buildTrackLookup returns a function that loads the track metadata
// for a given match. The MVP plays every match on a tiny 5-meter
// square ("MVP-Loop") so the race loop can complete end-to-end
// without per-track steering logic. We re-use the FIRST track row
// from Postgres (so the FK on races.track_id resolves) but ignore
// its layout — once per-track driving is implemented the layout
// will be loaded from the JSONB column.
func buildTrackLookup(p *pgxpool.Pool) race.TrackLookup {
	var cachedID uuid.UUID
	var cachedName string
	if p != nil {
		_ = p.QueryRow(context.Background(),
			`SELECT id, name FROM tracks ORDER BY name LIMIT 1`,
		).Scan(&cachedID, &cachedName)
	}
	if cachedID == uuid.Nil {
		// DB unavailable — fall back to a stable synthetic UUID.
		// Persistence will fail because of the FK, but the race
		// itself still runs.
		cachedID = uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
		cachedName = "MVP-Loop"
	}
	finalID := cachedID
	finalName := cachedName
	return func(_ uuid.UUID) (*race.Track, error) {
		return &race.Track{
			ID:   finalID,
			Name: finalName,
			Waypoints: []race.Waypoint{
				{X: 0, Z: 0}, {X: 5, Z: 0}, {X: 5, Z: 5}, {X: 0, Z: 5},
			},
		}, nil
	}
}

// broadcasterPool keeps a Broadcaster per race and a single goroutine
// that polls the race manager for new races.
type broadcasterPool struct {
	races    *race.Manager
	sessions *networking.SessionRegistry
	writer   networking.UDPWriter
	log      *slog.Logger

	mu        sync.Mutex
	known     map[uuid.UUID]bool
}

func newBroadcasterPool(rm *race.Manager, s *networking.SessionRegistry, w networking.UDPWriter, log *slog.Logger) *broadcasterPool {
	return &broadcasterPool{
		races:    rm,
		sessions: s,
		writer:   w,
		log:      log,
		known:    map[uuid.UUID]bool{},
	}
}

func (p *broadcasterPool) wireNewRaces(ctx context.Context) {
	for _, r := range p.races.List() {
		p.spawn(ctx, r)
	}
}

func (p *broadcasterPool) spawn(ctx context.Context, r *race.Race) {
	p.mu.Lock()
	if p.known[r.ID] {
		p.mu.Unlock()
		return
	}
	p.known[r.ID] = true
	p.mu.Unlock()

	subs := networking.NewSubscriptions()
	br := networking.NewBroadcaster(r, subs, p.writer, p.log)
	go br.Run(ctx)

	// Spin up a watcher that copies the session registry into the
	// subscription table for this race. When a player first appears
	// in the race we add their addr; when they leave we drop it.
	go p.followSubscriptions(ctx, r, subs)
}

func (p *broadcasterPool) followSubscriptions(ctx context.Context, r *race.Race, subs *networking.Subscriptions) {
	t := time.NewTicker(200 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-r.Done:
			return
		case <-t.C:
			for _, p := range p.sessions.RacePlayers(r) {
				if addr := p.Addr; addr != nil {
					subs.Add(p.PlayerID, addr)
				}
			}
		}
	}
}

func (p *broadcasterPool) watch(ctx context.Context) {
	t := time.NewTicker(500 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			for _, r := range p.races.List() {
				p.spawn(ctx, r)
			}
			p.races.CleanupFinished()
		}
	}
}

// watchRaceFinishes is retained as a no-op stub so callers using the
// older RaceManager signature still link. Persistence is now driven
// by the race's OnFinish hook.
func watchRaceFinishes(ctx context.Context, rm *race.Manager, w *results.Writer, log *slog.Logger) {
	<-ctx.Done()
}
