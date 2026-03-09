package config

import (
	"os"
	"path/filepath"
	"testing"
)

func clearEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"OPENROUTER_API_KEY", "OPENROUTER_BASE_URL",
		"SCAFFY_MODEL", "SCAFFY_MAX_TOKENS", "SCAFFY_TEMPERATURE",
		"SCAFFY_SYSTEM_PROMPT", "BRAVE_API_KEY",
	} {
		t.Setenv(key, "")
	}
}

func TestLoadDefaults(t *testing.T) {
	clearEnv(t)
	t.Setenv("OPENROUTER_API_KEY", "test-key")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.APIKey != "test-key" {
		t.Errorf("expected api key 'test-key', got %q", cfg.APIKey)
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
	if cfg.BashTimeout != 30 {
		t.Errorf("expected default bash timeout 30, got %d", cfg.BashTimeout)
	}
	if cfg.SystemPrompt != "" {
		t.Errorf("expected empty system prompt, got %q", cfg.SystemPrompt)
	}
	if len(cfg.Tools) != 9 {
		t.Errorf("expected 9 default tools, got %v", cfg.Tools)
	}
}

func TestLoadFromYAML(t *testing.T) {
	clearEnv(t)

	dir := t.TempDir()
	yaml := `api_key: "sk-yaml-key"
base_url: "https://custom.api/v1"
model: "gpt-4"
max_tokens: 8192
temperature: 0.5
system_prompt: "You are helpful."
bash_timeout: 60
brave_api_key: "BSA-yaml"
tools:
  - echo
  - read_file
  - bash_exec
`
	if err := os.WriteFile(filepath.Join(dir, "scaffy.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	// chdir to temp dir so Load() finds scaffy.yaml
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.APIKey != "sk-yaml-key" {
		t.Errorf("expected api key from YAML, got %q", cfg.APIKey)
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
	if cfg.BashTimeout != 60 {
		t.Errorf("expected bash timeout 60, got %d", cfg.BashTimeout)
	}
	if cfg.BraveAPIKey != "BSA-yaml" {
		t.Errorf("expected brave api key, got %q", cfg.BraveAPIKey)
	}
	if len(cfg.Tools) != 3 || cfg.Tools[0] != "echo" || cfg.Tools[1] != "read_file" || cfg.Tools[2] != "bash_exec" {
		t.Errorf("expected 3 tools, got %v", cfg.Tools)
	}
}

func TestEnvOverridesYAML(t *testing.T) {
	clearEnv(t)

	dir := t.TempDir()
	yaml := `api_key: "sk-yaml-key"
model: "yaml-model"
`
	if err := os.WriteFile(filepath.Join(dir, "scaffy.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	t.Setenv("OPENROUTER_API_KEY", "sk-env-key")
	t.Setenv("SCAFFY_MODEL", "env-model")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.APIKey != "sk-env-key" {
		t.Errorf("env should override YAML api key, got %q", cfg.APIKey)
	}
	if cfg.Model != "env-model" {
		t.Errorf("env should override YAML model, got %q", cfg.Model)
	}
}

func TestLoadFromEnv(t *testing.T) {
	clearEnv(t)
	t.Setenv("OPENROUTER_API_KEY", "sk-test")
	t.Setenv("OPENROUTER_BASE_URL", "https://custom.api/v1")
	t.Setenv("SCAFFY_MODEL", "gpt-4")
	t.Setenv("SCAFFY_MAX_TOKENS", "8192")
	t.Setenv("SCAFFY_TEMPERATURE", "0.5")
	t.Setenv("SCAFFY_SYSTEM_PROMPT", "You are helpful.")
	t.Setenv("BRAVE_API_KEY", "BSA-env")

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
	if cfg.BraveAPIKey != "BSA-env" {
		t.Errorf("expected brave api key, got %q", cfg.BraveAPIKey)
	}
}

func TestLoadMissingAPIKey(t *testing.T) {
	clearEnv(t)

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	clearEnv(t)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "scaffy.yaml"), []byte("{{invalid yaml"), 0o644); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}
