package main

import (
	"fmt"
	"log/slog"
	"os"

	tea "github.com/charmbracelet/bubbletea"

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
	var reasoning *llmclient.ReasoningConfig
	if cfg.Reasoning != nil {
		reasoning = &llmclient.ReasoningConfig{
			Effort:    cfg.Reasoning.Effort,
			MaxTokens: cfg.Reasoning.MaxTokens,
			Exclude:   cfg.Reasoning.Exclude,
		}
	}
	client := llmclient.NewLlmClient(
		cfg.APIKey,
		cfg.BaseURL,
		cfg.Model,
		cfg.MaxTokens,
		cfg.Temperature,
		reasoning,
	)

	// Map of all available tools
	allTools := map[string]tools.ToolHandler{
		"echo":           &tools.EchoTool{},
		"read_file":      &tools.ReadFileTool{},
		"write_file":     &tools.WriteFileTool{},
		"edit_file":      &tools.EditFileTool{},
		"list_directory": &tools.ListDirectoryTool{},
		"search_files":   &tools.SearchFilesTool{},
		"grep_search":    &tools.GrepSearchTool{},
		"bash_exec":      tools.NewBashExecTool(cfg.BashTimeout),
		"web_fetch":      &tools.WebFetchTool{},
		"web_search":     tools.NewWebSearchTool(cfg.BraveAPIKey),
	}

	// Register only tools listed in config
	registry := tools.NewRegistry()
	for _, name := range cfg.Tools {
		if handler, ok := allTools[name]; ok {
			registry.Register(handler)
		} else {
			slog.Warn("unknown tool in config, skipping", "tool", name)
		}
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
