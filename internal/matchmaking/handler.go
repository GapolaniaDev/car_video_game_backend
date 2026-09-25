package matchmaking

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gustavo/racing-game-backend/internal/auth"
)

// Handlers wires /matchmaking/join behind auth.Middleware.
type Handlers struct {
	svc *Service
	log *slog.Logger
}

func NewHandlers(svc *Service, log *slog.Logger) *Handlers {
	if log == nil {
		log = slog.Default()
	}
	return &Handlers{svc: svc, log: log}
}

// Join handles POST /api/v1/matchmaking/join. No body required.
func (h *Handlers) Join(w http.ResponseWriter, r *http.Request) {
	playerID, ok := auth.PlayerIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	info, err := h.svc.Join(r.Context(), playerID)
	if err != nil {
		switch err {
		case ErrNoGameServer, ErrRaceFull:
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusServiceUnavailable)
		default:
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		}
		h.log.Error("matchmaking.join.error", slog.String("error", err.Error()))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(info)
}