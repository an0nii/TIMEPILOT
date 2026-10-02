package config

import (
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("PORT", "9090")
	t.Setenv("HTTP_READ_TIMEOUT", "3s")
	t.Setenv("AI_PROVIDER", "disabled")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != "9090" || cfg.ReadTimeout != 3*time.Second {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}

func TestLoadAIProviderKey(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("AI_PROVIDER", "openai")
	t.Setenv("AI_MODEL", "test-model")
	t.Setenv("OPENAI_API_KEY", "secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AI.APIKey != "secret" || cfg.AI.Provider != "openai" {
		t.Fatalf("unexpected AI config: %+v", cfg.AI)
	}
}
