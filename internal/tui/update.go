package tui

import (
	"context"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	openai "github.com/sashabaranov/go-openai"

	"github.com/gallanoe/scaffy/internal/conversation"
	"github.com/gallanoe/scaffy/internal/llmclient"
	"github.com/gallanoe/scaffy/internal/tools"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg)

	case StreamTickMsg:
		return m.handleStreamTick(msg)

	case StreamDoneMsg:
		return m.handleStreamDone(msg)

	case ToolResultMsg:
		return m.handleToolResult(msg)

	case streamStartedMsg:
		if msg.Generation == m.streamGeneration {
			m.streamChan = msg.Chan
			return m, waitForStream(m.streamChan, m.streamGeneration)
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case statusTimeoutMsg:
		if time.Now().After(m.statusExpiry) {
			m.statusMsg = ""
		}
		return m, nil
	}

	// Pass through to textarea if focused
	if m.focus == FocusInput {
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.focus {
	case FocusInput:
		return m.handleInputKey(msg)
	case FocusHistory:
		return m.handleHistoryKey(msg)
	}
	return m, nil
}

func (m Model) handleInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		m.quitting = true
		return m, tea.Quit

	case tea.KeyEnter:
		// Alt+Enter for newline (Shift+Enter unreliable in some terminals)
		if msg.Alt {
			var cmd tea.Cmd
			m.textarea, cmd = m.textarea.Update(msg)
			return m, cmd
		}
		return m.sendMessage()

	case tea.KeyTab:
		m.focus = FocusHistory
		if !m.conversation.IsEmpty() && m.selectedMessage == nil {
			idx := nextVisibleMessage(m.conversation, m.conversation.Len()-1, -1)
			if idx >= 0 {
				m.selectedMessage = &idx
			}
		}
		return m, nil

	case tea.KeyEsc:
		if m.streamingState == StateError {
			m.streamingState = StateIdle
			m.errorMessage = ""
		}
		return m, nil

	default:
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		return m, cmd
	}
}

func (m Model) handleHistoryKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		m.quitting = true
		return m, tea.Quit

	case tea.KeyUp:
		if m.selectedMessage != nil && *m.selectedMessage > 0 {
			idx := nextVisibleMessage(m.conversation, *m.selectedMessage-1, -1)
			if idx >= 0 {
				m.selectedMessage = &idx
			}
		}
		return m, nil

	case tea.KeyDown:
		if m.selectedMessage != nil && *m.selectedMessage+1 < m.conversation.Len() {
			idx := nextVisibleMessage(m.conversation, *m.selectedMessage+1, 1)
			if idx >= 0 && idx < m.conversation.Len() {
				m.selectedMessage = &idx
			}
		}
		return m, nil

	case tea.KeyEnter:
		m.toggleSelectedExpand()
		return m, nil

	case tea.KeyTab:
		m.focus = FocusInput
		m.textarea.Focus()
		return m, nil

	case tea.KeyEsc:
		m.focus = FocusInput
		m.selectedMessage = nil
		m.textarea.Focus()
		return m, nil

	default:
		return m, nil
	}
}

func (m *Model) toggleSelectedExpand() {
	if m.selectedMessage == nil {
		return
	}
	idx := *m.selectedMessage
	if idx >= len(m.conversation.Messages) {
		return
	}
	msg := m.conversation.Messages[idx]
	id := msg.Metadata.ID
	if len(msg.ToolCalls) > 0 || msg.ToolResult != nil {
		if m.expandedBlocks[id] {
			delete(m.expandedBlocks, id)
		} else {
			m.expandedBlocks[id] = true
		}
	}
}

func (m Model) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height

	headerHeight := 0
	inputHeight := 5
	statusBarHeight := 1
	vpHeight := m.height - inputHeight - headerHeight - statusBarHeight
	if vpHeight < 1 {
		vpHeight = 1
	}

	if !m.viewportReady {
		m.viewport = viewport.New(m.width, vpHeight)
		m.viewportReady = true
	} else {
		m.viewport.Width = m.width
		m.viewport.Height = vpHeight
	}

	m.textarea.SetWidth(m.width)

	return m, nil
}

func (m Model) handleStreamTick(msg StreamTickMsg) (tea.Model, tea.Cmd) {
	if msg.Generation != m.streamGeneration {
		if m.streamChan != nil {
			return m, waitForStream(m.streamChan, m.streamGeneration)
		}
		return m, nil
	}

	event := msg.Event
	switch event.Type {
	case llmclient.StreamMsgReasoningToken:
		m.partialReasoning += event.Token
		return m, waitForStream(m.streamChan, m.streamGeneration)

	case llmclient.StreamMsgToken:
		m.partialContent += event.Token
		return m, waitForStream(m.streamChan, m.streamGeneration)

	case llmclient.StreamMsgToolCallComplete:
		m.finalizePartialContent()
		if event.ToolCall != nil {
			m.accumulatedToolCalls = append(m.accumulatedToolCalls, *event.ToolCall)
		}
		return m, waitForStream(m.streamChan, m.streamGeneration)

	case llmclient.StreamMsgDone:
		m.finalizePartialContent()
		if len(m.accumulatedToolCalls) > 0 {
			// Push one assistant message with all tool calls
			m.conversation.Push(conversation.NewAssistantToolCallsMessage(m.accumulatedToolCalls))

			// Populate pending set and fire parallel executions
			m.pendingToolCalls = make(map[string]bool)
			cmds := make([]tea.Cmd, 0, len(m.accumulatedToolCalls)+1)
			gen := m.streamGeneration
			registry := m.toolRegistry
			for _, call := range m.accumulatedToolCalls {
				m.pendingToolCalls[call.ID] = true
				cmds = append(cmds, executeToolCmd(registry, call, gen))
			}
			cmds = append(cmds, m.spinner.Tick)

			m.streamingState = StateToolsExecuting
			m.accumulatedToolCalls = nil
			m.streamChan = nil
			return m, tea.Batch(cmds...)
		}
		m.streamingState = StateIdle
		m.streamChan = nil
		return m, nil

	case llmclient.StreamMsgError:
		m.finalizePartialContent()
		m.streamingState = StateError
		m.errorMessage = event.Error
		m.streamChan = nil
		return m, nil

	default:
		return m, waitForStream(m.streamChan, m.streamGeneration)
	}
}

func (m Model) handleStreamDone(msg StreamDoneMsg) (tea.Model, tea.Cmd) {
	if msg.Generation != m.streamGeneration {
		return m, nil
	}
	// Don't reset to idle if we're waiting for tool results
	if m.streamingState == StateToolsExecuting {
		return m, nil
	}
	m.finalizePartialContent()
	m.streamingState = StateIdle
	m.streamChan = nil
	return m, nil
}

func (m Model) handleToolResult(msg ToolResultMsg) (tea.Model, tea.Cmd) {
	if msg.Generation != m.streamGeneration {
		return m, nil
	}

	// Push result to conversation (errors become result content)
	content := msg.Content
	if msg.Error != "" {
		content = "Error: " + msg.Error
	}
	m.conversation.Push(conversation.NewToolResultMessage(msg.ToolCallID, content))

	// Remove from pending set
	delete(m.pendingToolCalls, msg.ToolCallID)

	// If calls still pending, wait
	if len(m.pendingToolCalls) > 0 {
		return m, nil
	}

	// All resolved — start new LLM stream
	m.streamGeneration++
	m.streamingState = StateStreaming
	m.partialContent = ""
	m.partialReasoning = ""

	cmd := m.startStream()
	return m, tea.Batch(cmd, m.spinner.Tick)
}

func (m Model) sendMessage() (tea.Model, tea.Cmd) {
	if m.streamingState != StateIdle {
		return m, nil
	}

	text := strings.TrimSpace(m.textarea.Value())
	if text == "" {
		return m, nil
	}

	m.conversation.Push(conversation.NewUserMessage(text))
	m.textarea.Reset()

	m.streamGeneration++
	m.streamingState = StateStreaming
	m.partialContent = ""
	m.partialReasoning = ""
	m.accumulatedToolCalls = nil
	m.pendingToolCalls = make(map[string]bool)

	cmd := m.startStream()
	return m, tea.Batch(cmd, m.spinner.Tick)
}

func (m *Model) finalizePartialContent() {
	if m.partialContent != "" {
		m.conversation.Push(conversation.NewAssistantMessage(m.partialContent))
		m.partialContent = ""
	}
}

func (m Model) startStream() tea.Cmd {
	messages := m.conversation.ToOpenAIMessages()
	var openaiTools []openai.Tool
	if !m.toolRegistry.IsEmpty() {
		openaiTools = m.toolRegistry.ToOpenAITools()
	}

	client := m.llmClient
	gen := m.streamGeneration

	return func() tea.Msg {
		ch := client.ChatStream(context.Background(), messages, openaiTools)
		return streamStartedMsg{Generation: gen, Chan: ch}
	}
}

type streamStartedMsg struct {
	Generation uint64
	Chan       <-chan llmclient.StreamMsg
}

func waitForStream(ch <-chan llmclient.StreamMsg, gen uint64) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-ch
		if !ok {
			return StreamDoneMsg{Generation: gen}
		}
		return StreamTickMsg{Generation: gen, Event: event}
	}
}

// nextVisibleMessage finds the next message index from `from` in the given
// direction (1 or -1) that is not a RoleTool message. Returns -1 if none found.
func nextVisibleMessage(conv *conversation.Conversation, from, direction int) int {
	for i := from; i >= 0 && i < conv.Len(); i += direction {
		if conv.Messages[i].Role != conversation.RoleTool {
			return i
		}
	}
	return -1
}

func executeToolCmd(registry *tools.ToolRegistry, call conversation.ToolCall, gen uint64) tea.Cmd {
	return func() tea.Msg {
		result, err := registry.Execute(context.Background(), &call)
		if err != nil {
			return ToolResultMsg{
				Generation: gen,
				ToolCallID: call.ID,
				Error:      err.Error(),
			}
		}
		return ToolResultMsg{
			Generation: gen,
			ToolCallID: call.ID,
			Content:    result,
		}
	}
}
