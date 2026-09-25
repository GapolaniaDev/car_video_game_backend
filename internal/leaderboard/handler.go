package leaderboard

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gustavo/racing-game-backend/internal/auth"
)

// Handler serves GET /api/v1/leaderboard.
type Handler struct {
	repo *Repo
	log  *slog.Logger
}

// NewHandler wires a handler.
func NewHandler(repo *Repo, log *slog.Logger) *Handler {
	if log == nil {
		log = slog.Default()
	}
	return &Handler{repo: repo, log: log}
}

// Top handles GET /api/v1/leaderboard?trackId=<uuid>&limit=<n>.
func (h *Handler) Top(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.PlayerIDFromContext(r.Context()); !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	trackIDStr := r.URL.Query().Get("trackId")
	if trackIDStr == "" {
		http.Error(w, `{"error":"trackId is required"}`, http.StatusBadRequest)
		return
	}
	trackID, err := uuid.Parse(trackIDStr)
	if err != nil {
		http.Error(w, `{"error":"bad trackId uuid"}`, http.StatusBadRequest)
		return
	}
	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		n, err := strconv.Atoi(l)
		if err != nil || n <= 0 {
			http.Error(w, `{"error":"bad limit"}`, http.StatusBadRequest)
			return
		}
		if n > 100 {
			n = 100
		}
		limit = n
	}
	track, err := h.repo.LoadTrack(r.Context(), trackID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		h.log.Error("leaderboard.track.error", slog.String("err", err.Error()))
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	entries, err := h.repo.BestLapsByTrack(r.Context(), trackID, limit)
	if err != nil {
		h.log.Error("leaderboard.entries.error", slog.String("err", err.Error()))
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	if entries == nil {
		entries = []Entry{}
	}
	h.log.Info("leaderboard.served",
		slog.String("track_id", trackID.String()),
		slog.Int("limit", limit),
		slog.Int("returned_count", len(entries)),
	)
	type Response struct {
		TrackID uuid.UUID `json:"trackId"`
		Name    string    `json:"trackName"`
		Entries []Entry   `json:"entries"`
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(Response{
		TrackID: track.TrackID,
		Name:    track.Name,
		Entries: entries,
	})
}
