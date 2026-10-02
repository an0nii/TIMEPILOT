package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL     string
	Port            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
	AI              AIConfig
}

type AIConfig struct {
	Provider string
	APIKey   string
	Model    string
	BaseURL  string
	Timeout  time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		Port:        valueOrDefault("PORT", "8080"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	var err error
	if cfg.ReadTimeout, err = duration("HTTP_READ_TIMEOUT", 5*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.WriteTimeout, err = duration("HTTP_WRITE_TIMEOUT", 40*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.ShutdownTimeout, err = duration("SHUTDOWN_TIMEOUT", 10*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.AI, err = loadAI(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func loadAI() (AIConfig, error) {
	provider := strings.ToLower(valueOrDefault("AI_PROVIDER", "disabled"))
	if provider != "disabled" && provider != "openai" && provider != "anthropic" && provider != "gemini" {
		return AIConfig{}, fmt.Errorf("AI_PROVIDER must be disabled, openai, anthropic or gemini")
	}

	timeout, err := duration("AI_TIMEOUT", 30*time.Second)
	if err != nil {
		return AIConfig{}, err
	}
	config := AIConfig{
		Provider: provider,
		APIKey:   os.Getenv("AI_API_KEY"),
		Model:    os.Getenv("AI_MODEL"),
		BaseURL:  strings.TrimRight(os.Getenv("AI_BASE_URL"), "/"),
		Timeout:  timeout,
	}
	if provider == "disabled" {
		return config, nil
	}
	if config.APIKey == "" {
		config.APIKey = os.Getenv(map[string]string{
			"openai":    "OPENAI_API_KEY",
			"anthropic": "ANTHROPIC_API_KEY",
			"gemini":    "GEMINI_API_KEY",
		}[provider])
	}
	if config.APIKey == "" {
		return AIConfig{}, fmt.Errorf("AI_API_KEY or the provider-specific API key is required when AI_PROVIDER=%s", provider)
	}
	if config.Model == "" {
		return AIConfig{}, fmt.Errorf("AI_MODEL is required when AI_PROVIDER=%s", provider)
	}
	return config, nil
}

func valueOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func duration(name string, fallback time.Duration) (time.Duration, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	return parsed, nil
}
