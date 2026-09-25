package player

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gustavo/racing-game-backend/internal/auth"
)

// Handler serves /players/me. Auth middleware is expected to have
// already attached the player id to the context.
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

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	playerID, ok := auth.PlayerIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	profile, err := h.repo.GetByID(r.Context(), playerID)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(profile)
}