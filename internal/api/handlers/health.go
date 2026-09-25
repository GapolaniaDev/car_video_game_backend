// Package handlers contains HTTP handlers for the REST API.
package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/gustavo/racing-game-backend/internal/database"
)

// HealthDeps carries the dependencies needed by the health handler.
// DB may be nil — the handler reports database:"skipped" in that case.
type HealthDeps struct {
	DB  *database.Pool
	Log *slog.Logger
}

// HealthResponse is the JSON body returned by GET /api/v1/health.
type HealthResponse struct {
	Status   string `json:"status"`
	Service  string `json:"service"`
	Database string `json:"database,omitempty"`
}

// Health returns the handler for GET /api/v1/health. It does a best-
// effort DB ping with a short timeout so a healthy API can still
// respond when Postgres is briefly unavailable.
func Health(deps HealthDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := HealthResponse{Status: "ok", Service: "backend-api"}

		if deps.DB != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			if err := deps.DB.HealthCheck(ctx); err != nil {
				resp.Database = "down"
			} else {
				resp.Database = "ok"
			}
		} else {
			resp.Database = "skipped"
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}
}