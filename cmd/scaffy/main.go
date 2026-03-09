package main

import (
	"fmt"
	"log/slog"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joho/godotenv"

	"github.com/gallanoe/scaffy/internal/config"
	"github.com/gallanoe/scaffy/internal/conversation"
	"github.com/gallanoe/scaffy/internal/llmclient"
	"github.com/gallanoe/scaffy/internal/tools"
	"github.com/gallanoe/scaffy/internal/tui"
)

func main() {
	os.Exit(run())
}

func run() int {
	// Load .env (ignore error if missing)
	_ = godotenv.Load()

	// Init logging to file
	logFile, err := os.Create("scaffy.log")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create log file: %v\n", err)
		return 1
	}
	defer func() { _ = logFile.Close() }()
	slog.SetDefault(slog.New(slog.NewTextHandler(logFile, nil)))

	// Load config
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		return 1
	}

	// Create LLM client
	client := llmclient.NewLlmClient(
		cfg.APIKey,
		cfg.BaseURL,
		cfg.Model,
		cfg.MaxTokens,
		cfg.Temperature,
	)

	// Create tool registry
	registry := tools.NewRegistry()
	if cfg.EchoTool {
		registry.Register(&tools.EchoTool{})
	}

	// Create conversation and add system prompt
	conv := conversation.NewConversation()
	if cfg.SystemPrompt != "" {
		conv.Push(conversation.NewSystemMessage(cfg.SystemPrompt))
	}

	// Create and run TUI
	model := tui.NewModel(conv, registry, client, cfg.Model)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}
