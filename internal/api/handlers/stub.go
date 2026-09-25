package handlers

import (
	"encoding/json"
	"net/http"
)

// NotImplementedJSON is the body returned by stub endpoints.
type NotImplementedJSON struct {
	Error string `json:"error"`
	Path  string `json:"path"`
}

// Stub returns a 501 Not Implemented handler for endpoints that are
// registered but have not yet been built out. This keeps the API
// surface discoverable for clients (e.g. Unity) without committing
// to behavior we have not implemented.
func Stub(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotImplemented)
		_ = json.NewEncoder(w).Encode(NotImplementedJSON{
			Error: "not implemented",
			Path:  path,
		})
	}
}