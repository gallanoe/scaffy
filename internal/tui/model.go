package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"

	"github.com/gallanoe/scaffy/internal/conversation"
	"github.com/gallanoe/scaffy/internal/llmclient"
	"github.com/gallanoe/scaffy/internal/tools"
	"github.com/gallanoe/scaffy/internal/tui/markdown"
	"github.com/gallanoe/scaffy/internal/tui/styles"
)

type AppFocus int

const (
	FocusInput AppFocus = iota
	FocusHistory
)

type StreamingState int

const (
	StateIdle StreamingState = iota
	StateStreaming
	StateToolsExecuting
	StateError
)

type Model struct {
	conversation         *conversation.Conversation
	toolRegistry         *tools.ToolRegistry
	llmClient            *llmclient.LlmClient
	textarea             textarea.Model
	viewport             viewport.Model
	spinner              spinner.Model
	styles               *styles.Styles
	mdCache              *markdown.Cache
	modelName            string
	focus                AppFocus
	streamingState       StreamingState
	partialContent       string
	errorMessage         string
	statusMsg            string
	statusExpiry         time.Time
	streamGeneration     uint64
	selectedMessage      *int
	expandedBlocks       map[uuid.UUID]bool
	pendingToolCalls     map[string]bool
	accumulatedToolCalls []conversation.ToolCall
	quitting             bool
	streamChan           <-chan llmclient.StreamMsg
	width                int
	height               int
	viewportReady        bool
}

func NewModel(conv *conversation.Conversation, registry *tools.ToolRegistry, client *llmclient.LlmClient, modelName string) Model {
	ta := textarea.New()
	ta.Placeholder = "Type a message..."
	ta.Focus()
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.Prompt = ""
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()

	s := styles.NewStyles()

	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	sp.Style = s.Spinner

	return Model{
		conversation:     conv,
		toolRegistry:     registry,
		llmClient:        client,
		textarea:         ta,
		spinner:          sp,
		styles:           s,
		mdCache:          markdown.NewCache(),
		modelName:        modelName,
		focus:            FocusInput,
		streamingState:   StateIdle,
		expandedBlocks:   make(map[uuid.UUID]bool),
		pendingToolCalls: make(map[string]bool),
	}
}

func (m Model) Init() tea.Cmd {
	return textarea.Blink
}
