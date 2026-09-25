package garage

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gustavo/racing-game-backend/internal/auth"
)

// Handler serves /garage.
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

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	playerID, ok := auth.PlayerIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	out, err := h.repo.ListByPlayer(r.Context(), playerID)
	if err != nil {
		h.log.Error("garage.list.error", slog.String("error", err.Error()))
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	if out == nil {
		out = []Entry{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}