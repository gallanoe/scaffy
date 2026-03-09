package config

import (
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	// Clear all scaffy env vars, set only required
	t.Setenv("OPENROUTER_API_KEY", "test-key")
	t.Setenv("OPENROUTER_BASE_URL", "")
	t.Setenv("SCAFFY_MODEL", "")
	t.Setenv("SCAFFY_MAX_TOKENS", "")
	t.Setenv("SCAFFY_TEMPERATURE", "")
	t.Setenv("SCAFFY_SYSTEM_PROMPT", "")
	t.Setenv("SCAFFY_ECHO_TOOL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.BaseURL != "https://openrouter.ai/api/v1" {
		t.Errorf("expected default base URL, got %q", cfg.BaseURL)
	}
	if cfg.Model != "anthropic/claude-sonnet-4" {
		t.Errorf("expected default model, got %q", cfg.Model)
	}
	if cfg.MaxTokens != 4096 {
		t.Errorf("expected default max tokens 4096, got %d", cfg.MaxTokens)
	}
	if cfg.Temperature != 0.7 {
		t.Errorf("expected default temperature 0.7, got %f", cfg.Temperature)
	}
	if cfg.SystemPrompt != "" {
		t.Errorf("expected empty system prompt, got %q", cfg.SystemPrompt)
	}
	if cfg.EchoTool {
		t.Error("expected echo tool disabled by default")
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "sk-test")
	t.Setenv("OPENROUTER_BASE_URL", "https://custom.api/v1")
	t.Setenv("SCAFFY_MODEL", "gpt-4")
	t.Setenv("SCAFFY_MAX_TOKENS", "8192")
	t.Setenv("SCAFFY_TEMPERATURE", "0.5")
	t.Setenv("SCAFFY_SYSTEM_PROMPT", "You are helpful.")
	t.Setenv("SCAFFY_ECHO_TOOL", "1")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.APIKey != "sk-test" {
		t.Errorf("expected api key 'sk-test', got %q", cfg.APIKey)
	}
	if cfg.BaseURL != "https://custom.api/v1" {
		t.Errorf("expected custom base URL, got %q", cfg.BaseURL)
	}
	if cfg.Model != "gpt-4" {
		t.Errorf("expected model 'gpt-4', got %q", cfg.Model)
	}
	if cfg.MaxTokens != 8192 {
		t.Errorf("expected max tokens 8192, got %d", cfg.MaxTokens)
	}
	if cfg.Temperature != 0.5 {
		t.Errorf("expected temperature 0.5, got %f", cfg.Temperature)
	}
	if cfg.SystemPrompt != "You are helpful." {
		t.Errorf("expected system prompt, got %q", cfg.SystemPrompt)
	}
	if !cfg.EchoTool {
		t.Error("expected echo tool enabled")
	}
}

func TestLoadMissingAPIKey(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}
