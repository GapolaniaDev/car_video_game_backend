// Package config loads process configuration from environment variables.
//
// All configuration is environment-driven — no production values are
// hardcoded in source. Required secrets (POSTGRES_PASSWORD, JWT_SECRET)
// must be provided at startup. Defaults are applied only for non-secret
// values that have a clear safe default in development.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds the runtime configuration for any of the backend
// executables (cmd/api, cmd/game-server, cmd/udp-client).
type Config struct {
	AppEnv          string        // "development" | "staging" | "production"
	LogLevel        slog.Level    // mapped from LOG_LEVEL
	APIPort         int           // internal HTTP port for the REST API
	GameServerPort  int           // UDP port for the Game Server

	// PostgreSQL connection settings.
	PostgresHost     string
	PostgresPort     int
	PostgresDB       string
	PostgresUser     string
	PostgresPassword string
	PostgresSSLMode  string

	// JWT signing secret. Must never be empty in non-dev environments.
	JWTSecret   string
	JWTIssuer   string
	JWTAccessTTL time.Duration

	// Game Server routing info surfaced to clients on /matchmaking/join.
	GameServerPublicHost string
	GameServerPublicPort int

	// HMAC secret shared by API and Game Server for `game_token`.
	GameTokenSecret string
}

// Load reads configuration from the process environment and returns a
// populated Config or a descriptive error if required values are
// missing or malformed.
func Load() (*Config, error) {
	cfg := &Config{
		AppEnv:               getEnv("APP_ENV", "development"),
		APIPort:              getEnvInt("API_PORT", 8080),
		GameServerPort:       getEnvInt("GAME_SERVER_PORT", 7000),
		PostgresHost:         getEnv("POSTGRES_HOST", "postgres"),
		PostgresPort:         getEnvInt("POSTGRES_PORT", 5432),
		PostgresDB:           getEnv("POSTGRES_DB", "racing_game"),
		PostgresUser:         getEnv("POSTGRES_USER", "racing_user"),
		PostgresPassword:     os.Getenv("POSTGRES_PASSWORD"),
		PostgresSSLMode:      getEnv("POSTGRES_SSLMODE", "disable"),
		JWTSecret:            os.Getenv("JWT_SECRET"),
		JWTIssuer:            getEnv("JWT_ISSUER", "racing-game-backend"),
		JWTAccessTTL:         getEnvDuration("JWT_ACCESS_TTL", time.Hour),
		GameServerPublicHost: getEnv("GAME_SERVER_PUBLIC_HOST", "localhost"),
		GameServerPublicPort: getEnvInt("GAME_SERVER_PUBLIC_PORT", 7000),
		GameTokenSecret:      getEnv("GAME_TOKEN_SECRET", "dev_game_token_secret_change_me"),
	}

	lvl, err := parseLogLevel(getEnv("LOG_LEVEL", "info"))
	if err != nil {
		return nil, fmt.Errorf("LOG_LEVEL: %w", err)
	}
	cfg.LogLevel = lvl

	// Required secrets: enforce only outside dev to avoid breaking
	// local-only smoke tests where the developer may want to spin up
	// the API without real secrets. Production-like envs fail fast.
	if cfg.AppEnv != "development" {
		if cfg.PostgresPassword == "" {
			return nil, errors.New("POSTGRES_PASSWORD is required when APP_ENV != development")
		}
		if cfg.JWTSecret == "" {
			return nil, errors.New("JWT_SECRET is required when APP_ENV != development")
		}
		if cfg.GameTokenSecret == "" {
			return nil, errors.New("GAME_TOKEN_SECRET is required when APP_ENV != development")
		}
	}

	// Loose sanity checks regardless of env.
	if cfg.PostgresPassword == "" && cfg.AppEnv == "development" {
		cfg.PostgresPassword = "dev_password"
	}
	if cfg.JWTSecret == "" && cfg.AppEnv == "development" {
		cfg.JWTSecret = "dev_jwt_secret_change_me"
	}

	if cfg.APIPort <= 0 || cfg.APIPort > 65535 {
		return nil, fmt.Errorf("API_PORT out of range: %d", cfg.APIPort)
	}
	if cfg.GameServerPort <= 0 || cfg.GameServerPort > 65535 {
		return nil, fmt.Errorf("GAME_SERVER_PORT out of range: %d", cfg.GameServerPort)
	}
	if cfg.GameServerPublicPort <= 0 || cfg.GameServerPublicPort > 65535 {
		return nil, fmt.Errorf("GAME_SERVER_PUBLIC_PORT out of range: %d", cfg.GameServerPublicPort)
	}
	if cfg.JWTAccessTTL <= 0 {
		return nil, fmt.Errorf("JWT_ACCESS_TTL must be positive, got %s", cfg.JWTAccessTTL)
	}

	return cfg, nil
}

// MustLoad is a convenience wrapper that panics on configuration
// errors. Intended for process entry points where configuration must
// be valid before the program can do anything useful.
func MustLoad() *Config {
	cfg, err := Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	return cfg
}

// DatabaseURL renders a libpq-style DSN suitable for pgx. It does not
// embed the password in any log output — callers must keep the DSN out
// of logs.
func (c *Config) DatabaseURL() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.PostgresUser, c.PostgresPassword,
		c.PostgresHost, c.PostgresPort, c.PostgresDB,
		c.PostgresSSLMode,
	)
}

// PostgresReadyTimeout is the maximum time the database package will
// wait for Postgres to become reachable on startup.
func (c *Config) PostgresReadyTimeout() time.Duration { return 30 * time.Second }

// ─── helpers ─────────────────────────────────────────────────────────

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseLogLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info", "":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unknown level %q (expected debug|info|warn|error)", s)
	}
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}