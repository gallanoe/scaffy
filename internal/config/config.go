package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	APIKey       string
	BaseURL      string
	Model        string
	MaxTokens    int
	Temperature  float64
	SystemPrompt string
	EchoTool     bool
}

func Load() (*Config, error) {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENROUTER_API_KEY must be set in .env or environment")
	}

	baseURL := os.Getenv("OPENROUTER_BASE_URL")
	if baseURL == "" {
		baseURL = "https://openrouter.ai/api/v1"
	}

	model := os.Getenv("SCAFFY_MODEL")
	if model == "" {
		model = "anthropic/claude-sonnet-4"
	}

	maxTokens := 4096
	if v := os.Getenv("SCAFFY_MAX_TOKENS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			maxTokens = parsed
		}
	}

	temperature := 0.7
	if v := os.Getenv("SCAFFY_TEMPERATURE"); v != "" {
		if parsed, err := strconv.ParseFloat(v, 64); err == nil {
			temperature = parsed
		}
	}

	systemPrompt := os.Getenv("SCAFFY_SYSTEM_PROMPT")
	echoTool := os.Getenv("SCAFFY_ECHO_TOOL") == "1"

	return &Config{
		APIKey:       apiKey,
		BaseURL:      baseURL,
		Model:        model,
		MaxTokens:    maxTokens,
		Temperature:  temperature,
		SystemPrompt: systemPrompt,
		EchoTool:     echoTool,
	}, nil
}
