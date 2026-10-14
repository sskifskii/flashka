package config

import (
	"testing"
)

func TestConfigValidation(t *testing.T) {
	valid := Config{HTTPAddr: ":8080", DatabaseURL: "postgres://x", CookieName: "session", SessionTTL: 3600000000000, LastSeenInterval: 60000000000, ArgonMemory: 19456, ArgonIterations: 2, ArgonParallelism: 1, TrustedOrigin: "https://example.com", Environment: "production"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid config: %v", err)
	}
	invalid := valid
	invalid.ArgonMemory = 19455
	if invalid.Validate() == nil {
		t.Fatal("weak argon memory accepted")
	}
	invalid = valid
	invalid.TrustedOrigin = "https://example.com/path"
	if invalid.Validate() == nil {
		t.Fatal("origin path accepted")
	}
}

func TestLoadNormalizesKnownEnvironment(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("APP_ENV", " Production ")
	t.Setenv("TRUSTED_ORIGIN", "https://example.com")
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Environment != "production" {
		t.Fatalf("environment=%q", got.Environment)
	}
}
func TestConfigRejectsUnknownEnvironment(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("APP_ENV", "prodution")
	t.Setenv("TRUSTED_ORIGIN", "https://example.com")
	if _, err := Load(); err == nil {
		t.Fatal("environment typo accepted")
	}
}
