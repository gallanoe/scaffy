package config

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	APIKey       string   `yaml:"api_key"`
	BaseURL      string   `yaml:"base_url"`
	Model        string   `yaml:"model"`
	MaxTokens    int      `yaml:"max_tokens"`
	Temperature  float64  `yaml:"temperature"`
	SystemPrompt string   `yaml:"system_prompt"`
	BashTimeout  int      `yaml:"bash_timeout"`
	BraveAPIKey  string   `yaml:"brave_api_key"`
	Tools        []string `yaml:"tools"`
}

func Load() (*Config, error) {
	cfg := &Config{}

	// Load YAML file if it exists
	data, err := os.ReadFile("scaffy.yaml")
	if err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("invalid scaffy.yaml: %w", err)
		}
	}

	// Apply defaults for unset fields
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://openrouter.ai/api/v1"
	}
	if cfg.Model == "" {
		cfg.Model = "anthropic/claude-sonnet-4"
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 4096
	}
	if cfg.Temperature == 0 {
		cfg.Temperature = 0.7
	}
	if cfg.BashTimeout == 0 {
		cfg.BashTimeout = 30
	}
	if len(cfg.Tools) == 0 {
		cfg.Tools = []string{
			"read_file", "write_file", "edit_file",
			"list_directory", "search_files", "grep_search",
			"bash_exec", "web_fetch", "web_search",
		}
	}

	// Env var overrides
	if v := os.Getenv("OPENROUTER_API_KEY"); v != "" {
		cfg.APIKey = v
	}
	if v := os.Getenv("OPENROUTER_BASE_URL"); v != "" {
		cfg.BaseURL = v
	}
	if v := os.Getenv("SCAFFY_MODEL"); v != "" {
		cfg.Model = v
	}
	if v := os.Getenv("SCAFFY_MAX_TOKENS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			cfg.MaxTokens = parsed
		}
	}
	if v := os.Getenv("SCAFFY_TEMPERATURE"); v != "" {
		if parsed, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.Temperature = parsed
		}
	}
	if v := os.Getenv("SCAFFY_SYSTEM_PROMPT"); v != "" {
		cfg.SystemPrompt = v
	}
	if v := os.Getenv("BRAVE_API_KEY"); v != "" {
		cfg.BraveAPIKey = v
	}

	// Validate required fields
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("api_key must be set in scaffy.yaml or OPENROUTER_API_KEY environment variable")
	}

	return cfg, nil
}
