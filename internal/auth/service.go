package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Service composes Repo + password hashing + JWT issuance.
type Service struct {
	repo   *Repo
	secret string
	issuer string
	ttl    time.Duration
	log    *slog.Logger
}

// NewService builds a Service. log may be nil (defaults to slog.Default()).
func NewService(repo *Repo, secret, issuer string, ttl time.Duration, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{repo: repo, secret: secret, issuer: issuer, ttl: ttl, log: log}
}

// RegisterRequest is the validated input to Register.
type RegisterRequest struct {
	Email       string
	Password    string
	DisplayName string
}

// Register creates a user and a player, returns access token + player id.
func (s *Service) Register(ctx context.Context, req RegisterRequest) (string, uuid.UUID, error) {
	email, err := normalizeEmail(req.Email)
	if err != nil {
		return "", uuid.Nil, fmt.Errorf("%w: %s", ErrInvalidInput, err)
	}
	if err := validatePassword(req.Password); err != nil {
		return "", uuid.Nil, fmt.Errorf("%w: %s", ErrInvalidInput, err)
	}
	name, err := validateDisplayName(req.DisplayName)
	if err != nil {
		return "", uuid.Nil, fmt.Errorf("%w: %s", ErrInvalidInput, err)
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		return "", uuid.Nil, fmt.Errorf("hash: %w", err)
	}

	userID, playerID, err := s.repo.CreateUserWithPlayer(ctx, email, hash, name)
	if err != nil {
		return "", uuid.Nil, err
	}

	tok, err := IssueAccessToken(s.secret, s.issuer, s.ttl, userID, playerID)
	if err != nil {
		return "", uuid.Nil, fmt.Errorf("issue token: %w", err)
	}
	s.log.Info("auth.register.ok",
		slog.String("player_id", playerID.String()),
		slog.String("user_id", userID.String()),
	)
	return tok, playerID, nil
}

// Login validates credentials and returns an access token + player id.
func (s *Service) Login(ctx context.Context, email, password string) (string, uuid.UUID, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		// Deliberately collapse to ErrInvalidCredentials so the caller
		// cannot distinguish "no such email" from "bad password".
		return "", uuid.Nil, ErrInvalidCredentials
	}
	userID, hash, err := s.repo.LookupUserByEmail(ctx, email)
	if err != nil {
		s.log.Info("auth.login.no_user", slog.String("email", email))
		return "", uuid.Nil, ErrInvalidCredentials
	}
	if err := VerifyPassword(hash, password); err != nil {
		s.log.Info("auth.login.bad_password", slog.String("user_id", userID.String()))
		return "", uuid.Nil, ErrInvalidCredentials
	}
	playerID, err := s.repo.PlayerByUserID(ctx, userID)
	if err != nil {
		return "", uuid.Nil, err
	}
	_ = s.repo.TouchLastLogin(ctx, userID)

	tok, err := IssueAccessToken(s.secret, s.issuer, s.ttl, userID, playerID)
	if err != nil {
		return "", uuid.Nil, fmt.Errorf("issue token: %w", err)
	}
	s.log.Info("auth.login.ok",
		slog.String("player_id", playerID.String()),
		slog.String("user_id", userID.String()),
	)
	return tok, playerID, nil
}

// Guest creates an anonymous account and returns access token + player id.
func (s *Service) Guest(ctx context.Context) (string, uuid.UUID, error) {
	name := "Guest-" + randomHex(3)
	userID, playerID, err := s.repo.CreateGuest(ctx, name)
	if err != nil {
		return "", uuid.Nil, err
	}
	tok, err := IssueAccessToken(s.secret, s.issuer, s.ttl, userID, playerID)
	if err != nil {
		return "", uuid.Nil, fmt.Errorf("issue token: %w", err)
	}
	s.log.Info("auth.guest.ok",
		slog.String("player_id", playerID.String()),
		slog.String("user_id", userID.String()),
	)
	return tok, playerID, nil
}

// ─── input validation ────────────────────────────────────────────────

func normalizeEmail(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", fmt.Errorf("email is required")
	}
	addr, err := mail.ParseAddress(s)
	if err != nil {
		return "", fmt.Errorf("invalid email")
	}
	return strings.ToLower(addr.Address), nil
}

func validatePassword(p string) error {
	if len(p) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	if len(p) > 128 {
		return fmt.Errorf("password too long")
	}
	return nil
}

func validateDisplayName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", fmt.Errorf("display name is required")
	}
	if len(trimmed) > 32 {
		return "", fmt.Errorf("display name must be at most 32 characters")
	}
	return trimmed, nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "000000"
	}
	return hex.EncodeToString(b)
}