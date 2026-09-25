package auth

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
)

// Handlers groups the HTTP endpoints for the auth package. Build
// with NewHandlers and route via http.ServeMux.
type Handlers struct {
	svc *Service
	log *slog.Logger
}

// NewHandlers constructs the handlers.
func NewHandlers(svc *Service, log *slog.Logger) *Handlers {
	if log == nil {
		log = slog.Default()
	}
	return &Handlers{svc: svc, log: log}
}

// ─── request / response shapes ──────────────────────────────────────

type registerRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthResponse is the common 200 body for register/login/guest.
type AuthResponse struct {
	AccessToken string `json:"accessToken"`
	PlayerID    string `json:"playerId"`
}

type errorBody struct {
	Error string `json:"error"`
}

// ─── handlers ───────────────────────────────────────────────────────

// Register handles POST /api/v1/auth/register.
func (h *Handlers) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid json")
		return
	}
	tok, playerID, err := h.svc.Register(r.Context(), RegisterRequest{
		Email:       req.Email,
		Password:    req.Password,
		DisplayName: req.DisplayName,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrEmailTaken):
			respondError(w, http.StatusConflict, "email already in use")
		case errors.Is(err, ErrInvalidInput):
			respondError(w, http.StatusBadRequest, err.Error())
		default:
			h.log.Error("auth.register.error", slog.String("error", err.Error()))
			respondError(w, http.StatusInternalServerError, "internal")
		}
		return
	}
	writeAuthResponse(w, tok, playerID)
}

// Login handles POST /api/v1/auth/login. Always returns the same
// generic message on bad credentials to avoid leaking whether the
// email exists.
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid json")
		return
	}
	tok, playerID, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			respondError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		h.log.Error("auth.login.error", slog.String("error", err.Error()))
		respondError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeAuthResponse(w, tok, playerID)
}

// Guest handles POST /api/v1/auth/guest. Body is empty.
func (h *Handlers) Guest(w http.ResponseWriter, r *http.Request) {
	tok, playerID, err := h.svc.Guest(r.Context())
	if err != nil {
		h.log.Error("auth.guest.error", slog.String("error", err.Error()))
		respondError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeAuthResponse(w, tok, playerID)
}

// ─── helpers ────────────────────────────────────────────────────────

func writeAuthResponse(w http.ResponseWriter, token string, playerID any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(AuthResponse{
		AccessToken: token,
		PlayerID:    toString(playerID),
	})
}

func respondError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorBody{Error: msg})
}

func toString(v any) string {
	if s, ok := v.(interface{ String() string }); ok {
		return s.String()
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}