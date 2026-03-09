package tui

import (
	"encoding/json"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gallanoe/scaffy/internal/conversation"
	"github.com/gallanoe/scaffy/internal/llmclient"
	"github.com/gallanoe/scaffy/internal/tools"
)

func newTestModel() Model {
	conv := conversation.NewConversation()
	registry := tools.NewRegistry()
	// No real client needed for unit tests — we test model state transitions
	return NewModel(conv, registry, nil, "test-model")
}

func updateModel(m Model, msg tea.Msg) Model {
	result, _ := m.Update(msg)
	return result.(Model)
}

func TestQuitOnCtrlC(t *testing.T) {
	m := newTestModel()

	m = updateModel(m, tea.KeyMsg{Type: tea.KeyCtrlC})

	if !m.quitting {
		t.Error("expected quitting to be true after Ctrl+C")
	}
}

func TestQuitOnCtrlCFromHistory(t *testing.T) {
	m := newTestModel()
	m.focus = FocusHistory

	m = updateModel(m, tea.KeyMsg{Type: tea.KeyCtrlC})

	if !m.quitting {
		t.Error("expected quitting to be true after Ctrl+C in history")
	}
}

func TestTabSwitchesFocus(t *testing.T) {
	m := newTestModel()
	m.conversation.Push(conversation.NewUserMessage("hi"))

	if m.focus != FocusInput {
		t.Fatal("expected initial focus on input")
	}

	// Tab to history
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.focus != FocusHistory {
		t.Error("expected focus on history after Tab")
	}
	if m.selectedMessage == nil {
		t.Error("expected selectedMessage to be set")
	}

	// Tab back to input
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.focus != FocusInput {
		t.Error("expected focus on input after second Tab")
	}
}

func TestEscDismissesError(t *testing.T) {
	m := newTestModel()
	m.streamingState = StateError
	m.errorMessage = "test error"

	m = updateModel(m, tea.KeyMsg{Type: tea.KeyEsc})

	if m.streamingState != StateIdle {
		t.Error("expected state to be idle after Esc")
	}
	if m.errorMessage != "" {
		t.Error("expected error message to be cleared")
	}
}

func TestEscFromHistoryReturnsToInput(t *testing.T) {
	m := newTestModel()
	m.focus = FocusHistory
	idx := 0
	m.selectedMessage = &idx

	m = updateModel(m, tea.KeyMsg{Type: tea.KeyEsc})

	if m.focus != FocusInput {
		t.Error("expected focus on input after Esc from history")
	}
	if m.selectedMessage != nil {
		t.Error("expected selectedMessage to be nil after Esc")
	}
}

func TestViewContainsSpinnerWhenStreaming(t *testing.T) {
	m := newTestModel()
	m.streamingState = StateStreaming
	m.partialContent = "Hello"

	view := m.View()

	if !containsString(view, "Thinking...") {
		t.Error("expected 'Thinking...' spinner label in view")
	}
	if !containsString(view, "Hello") {
		t.Error("expected partial content in view")
	}
}

func TestViewRendersMessages(t *testing.T) {
	m := newTestModel()
	m.conversation.Push(conversation.NewUserMessage("What is Go?"))
	m.conversation.Push(conversation.NewAssistantMessage("Go is a programming language."))

	view := m.View()

	if !containsString(view, "What is Go?") {
		t.Error("expected user message in view")
	}
	if !containsString(view, "programming") || !containsString(view, "language") {
		t.Error("expected assistant message in view")
	}
}

func TestViewRendersToolCalls(t *testing.T) {
	m := newTestModel()
	calls := []conversation.ToolCall{{
		ID:        "call_1",
		Name:      "echo",
		Arguments: json.RawMessage(`{"message":"hi"}`),
	}}
	m.conversation.Push(conversation.NewAssistantToolCallsMessage(calls))

	view := m.View()

	if !containsString(view, "echo") {
		t.Error("expected tool name in view")
	}
}

func TestViewRendersToolResults(t *testing.T) {
	m := newTestModel()
	m.conversation.Push(conversation.NewToolResultMessage("call_1", "echo result"))

	view := m.View()

	if !containsString(view, "Result:") {
		t.Error("expected result label in view")
	}
	if !containsString(view, "echo result") {
		t.Error("expected tool result content in view")
	}
}

func TestSendEmptyMessage(t *testing.T) {
	m := newTestModel()
	// textarea is empty by default

	m = updateModel(m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.conversation.Len() != 0 {
		t.Error("expected no messages after sending empty input")
	}
	if m.streamingState != StateIdle {
		t.Error("expected state to remain idle")
	}
}

func TestGenerationFiltering(t *testing.T) {
	m := newTestModel()
	m.streamGeneration = 5
	m.streamingState = StateStreaming

	// Stale stream tick (generation 3, current is 5)
	staleMsg := StreamTickMsg{
		Generation: 3,
		Event: llmclient.StreamMsg{
			Type:  llmclient.StreamMsgToken,
			Token: "stale token",
		},
	}
	m = updateModel(m, staleMsg)

	if m.partialContent != "" {
		t.Error("expected stale token to be discarded")
	}

	// Current generation tick
	currentMsg := StreamTickMsg{
		Generation: 5,
		Event: llmclient.StreamMsg{
			Type:  llmclient.StreamMsgToken,
			Token: "current token",
		},
	}
	// Need a stream channel for this to work
	ch := make(chan llmclient.StreamMsg, 1)
	m.streamChan = ch
	m = updateModel(m, currentMsg)

	if m.partialContent != "current token" {
		t.Errorf("expected 'current token', got %q", m.partialContent)
	}
}

func TestHistoryNavigation(t *testing.T) {
	m := newTestModel()
	m.conversation.Push(conversation.NewUserMessage("first"))
	m.conversation.Push(conversation.NewAssistantMessage("second"))
	m.conversation.Push(conversation.NewUserMessage("third"))

	// Switch to history
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.selectedMessage == nil || *m.selectedMessage != 2 {
		t.Fatal("expected selection at last message (idx 2)")
	}

	// Navigate up
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyUp})
	if *m.selectedMessage != 1 {
		t.Errorf("expected selection at idx 1, got %d", *m.selectedMessage)
	}

	// Navigate up again
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyUp})
	if *m.selectedMessage != 0 {
		t.Errorf("expected selection at idx 0, got %d", *m.selectedMessage)
	}

	// Can't go above 0
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyUp})
	if *m.selectedMessage != 0 {
		t.Errorf("expected selection to stay at 0, got %d", *m.selectedMessage)
	}

	// Navigate down
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyDown})
	if *m.selectedMessage != 1 {
		t.Errorf("expected selection at idx 1, got %d", *m.selectedMessage)
	}
}

func TestToggleExpandToolBlock(t *testing.T) {
	m := newTestModel()
	calls := []conversation.ToolCall{{
		ID:        "call_1",
		Name:      "echo",
		Arguments: json.RawMessage(`{"message":"hi"}`),
	}}
	m.conversation.Push(conversation.NewAssistantToolCallsMessage(calls))

	msgID := m.conversation.Messages[0].Metadata.ID

	// Switch to history, select the tool call message
	m.focus = FocusHistory
	idx := 0
	m.selectedMessage = &idx

	// Toggle expand
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.expandedBlocks[msgID] {
		t.Error("expected block to be expanded")
	}

	// Toggle collapse
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.expandedBlocks[msgID] {
		t.Error("expected block to be collapsed")
	}
}

func TestViewHeader(t *testing.T) {
	m := newTestModel()
	view := m.View()

	if !containsString(view, "scaffy") {
		t.Error("expected 'scaffy' in header")
	}
	if !containsString(view, "test-model") {
		t.Error("expected model name in header")
	}
}

func TestViewStatusBar(t *testing.T) {
	m := newTestModel()
	view := m.View()

	if !containsString(view, "Tab") {
		t.Error("expected Tab hint in status bar")
	}
	if !containsString(view, "Enter") {
		t.Error("expected Enter hint in status bar")
	}
	if !containsString(view, "Ctrl+C") {
		t.Error("expected Ctrl+C hint in status bar")
	}
}

func TestViewErrorInStatusBar(t *testing.T) {
	m := newTestModel()
	m.streamingState = StateError
	m.errorMessage = "connection failed"

	view := m.View()

	if !containsString(view, "Error:") {
		t.Error("expected error label in status bar")
	}
	if !containsString(view, "connection failed") {
		t.Error("expected error message in status bar")
	}
	if !containsString(view, "Esc to dismiss") {
		t.Error("expected dismiss hint in status bar")
	}
}

func TestStreamDoneFinalizesContent(t *testing.T) {
	m := newTestModel()
	m.streamGeneration = 1
	m.streamingState = StateStreaming
	m.partialContent = "Hello world"

	m = updateModel(m, StreamDoneMsg{Generation: 1})

	if m.streamingState != StateIdle {
		t.Error("expected idle state after stream done")
	}
	if m.conversation.Len() != 1 {
		t.Fatalf("expected 1 message after finalize, got %d", m.conversation.Len())
	}
	if m.conversation.Messages[0].Content != "Hello world" {
		t.Errorf("expected finalized content 'Hello world', got %q", m.conversation.Messages[0].Content)
	}
}

// stripANSI removes ANSI escape sequences so substring checks work
// regardless of styling inserted by glamour/lipgloss.
func stripANSI(s string) string {
	var out []byte
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			// Skip until 'm' (SGR terminator) or end of string
			j := i + 2
			for j < len(s) && s[j] != 'm' {
				j++
			}
			if j < len(s) {
				j++ // skip 'm'
			}
			i = j
		} else {
			out = append(out, s[i])
			i++
		}
	}
	return string(out)
}

func containsString(s, substr string) bool {
	plain := stripANSI(s)
	return strings.Contains(plain, substr)
}
