package tui

import (
	"github.com/gallanoe/scaffy/internal/llmclient"
)

// StreamTickMsg carries one event from the LLM stream.
type StreamTickMsg struct {
	Generation uint64
	Event      llmclient.StreamMsg
}

// StreamDoneMsg signals the stream channel was closed.
type StreamDoneMsg struct {
	Generation uint64
}

// ToolResultMsg carries the result of a tool execution.
type ToolResultMsg struct {
	Generation uint64
	ToolCallID string
	Content    string
	Error      string
}

// statusTimeoutMsg signals that a timed status message has expired.
type statusTimeoutMsg struct{}
