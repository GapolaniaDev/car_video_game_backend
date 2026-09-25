package cars

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Handler serves /cars and /cars/{id}.
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

// List handles GET /api/v1/cars.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	out, err := h.repo.List(r.Context())
	if err != nil {
		h.log.Error("cars.list.error", slog.String("error", err.Error()))
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	if out == nil {
		out = []Car{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// Get handles GET /api/v1/cars/{id}. id is captured via PathValue
// because the route uses "{id}" as a path segment.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, `{"error":"bad uuid"}`, http.StatusBadRequest)
		return
	}
	c, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		h.log.Error("cars.get.error", slog.String("error", err.Error()))
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(c)
}