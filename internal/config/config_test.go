package config

import (
	"testing"
)

// withEnv sets an env var for the duration of the test and restores it.
func setEnv(t *testing.T, key, value string) {
	t.Helper()
	t.Setenv(key, value)
}

// TestLoadDefaults checks that defaults are applied for non-secret vars
// when only the required secret vars are set.
func TestLoadDefaults(t *testing.T) {
	setEnv(t, "APP_ENV", "development")
	setEnv(t, "POSTGRES_PASSWORD", "x")
	setEnv(t, "JWT_SECRET", "x")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.APIPort != 8080 {
		t.Errorf("default API_PORT = %d, want 8080", cfg.APIPort)
	}
	if cfg.GameServerPort != 7000 {
		t.Errorf("default GAME_SERVER_PORT = %d, want 7000", cfg.GameServerPort)
	}
	if cfg.PostgresHost != "postgres" {
		t.Errorf("default POSTGRES_HOST = %q, want \"postgres\"", cfg.PostgresHost)
	}
	if cfg.PostgresDB != "racing_game" {
		t.Errorf("default POSTGRES_DB = %q, want \"racing_game\"", cfg.PostgresDB)
	}
}

// TestLoadRequiresSecretsInProduction verifies the fail-fast behavior.
func TestLoadRequiresSecretsInProduction(t *testing.T) {
	setEnv(t, "APP_ENV", "production")
	// Intentionally do NOT set JWT_SECRET or POSTGRES_PASSWORD.
	if _, err := Load(); err == nil {
		t.Fatal("Load should fail when required secrets missing in production")
	}
}

// TestLoadHonorsExplicitOverrides ensures env overrides win.
func TestLoadHonorsExplicitOverrides(t *testing.T) {
	setEnv(t, "APP_ENV", "development")
	setEnv(t, "POSTGRES_PASSWORD", "x")
	setEnv(t, "JWT_SECRET", "x")
	setEnv(t, "API_PORT", "9090")
	setEnv(t, "POSTGRES_HOST", "custom-host")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.APIPort != 9090 {
		t.Errorf("API_PORT override not applied: got %d", cfg.APIPort)
	}
	if cfg.PostgresHost != "custom-host" {
		t.Errorf("POSTGRES_HOST override not applied: got %q", cfg.PostgresHost)
	}
}

// TestDatabaseURL checks DSN composition.
func TestDatabaseURL(t *testing.T) {
	setEnv(t, "APP_ENV", "development")
	setEnv(t, "POSTGRES_PASSWORD", "x")
	setEnv(t, "JWT_SECRET", "x")
	setEnv(t, "POSTGRES_HOST", "db.example.com")
	setEnv(t, "POSTGRES_PORT", "5433")
	setEnv(t, "POSTGRES_DB", "gamedb")
	setEnv(t, "POSTGRES_USER", "alice")
	setEnv(t, "POSTGRES_SSLMODE", "require")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := cfg.DatabaseURL()
	want := "postgres://alice:x@db.example.com:5433/gamedb?sslmode=require"
	if got != want {
		t.Errorf("DatabaseURL() = %q, want %q", got, want)
	}
}