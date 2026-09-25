package racehistory

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/gustavo/racing-game-backend/internal/auth"
)

// Handler serves /players/me/races and /races/{id}.
type Handler struct {
	repo *Repo
	log  *slog.Logger
}

func NewHandler(repo *Repo, log *slog.Logger) *Handler {
	if log == nil {
		log = slog.Default()
	}
	return &Handler{repo: repo, log: log}
}

// PlayerRaces handles GET /api/v1/players/me/races?limit=&cursor=
func (h *Handler) PlayerRaces(w http.ResponseWriter, r *http.Request) {
	playerID, ok := auth.PlayerIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		n, err := strconv.Atoi(l)
		if err != nil || n <= 0 {
			http.Error(w, `{"error":"bad limit"}`, http.StatusBadRequest)
			return
		}
		limit = n
	}
	var cursor *time.Time
	if c := r.URL.Query().Get("cursor"); c != "" {
		t, err := time.Parse(time.RFC3339Nano, c)
		if err != nil {
			t2, err2 := time.Parse(time.RFC3339, c)
			if err2 != nil {
				http.Error(w, `{"error":"bad cursor"}`, http.StatusBadRequest)
				return
			}
			t = t2
		}
		cursor = &t
	}
	page, err := h.repo.ListByPlayer(r.Context(), playerID, cursor, limit)
	if err != nil {
		h.log.Error("racehistory.list.error", slog.String("err", err.Error()))
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(page)
}

// Race handles GET /api/v1/races/{id}.
func (h *Handler) Race(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.PlayerIDFromContext(r.Context()); !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, `{"error":"bad uuid"}`, http.StatusBadRequest)
		return
	}
	d, err := h.repo.GetRace(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		h.log.Error("racehistory.get.error", slog.String("err", err.Error()))
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(d)
}
