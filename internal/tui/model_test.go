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

	if containsString(view, "Thinking...") {
		t.Error("expected 'Thinking...' spinner to be absent when content is streaming")
	}
	if !containsString(view, "Hello") {
		t.Error("expected partial content in view")
	}
}

func TestViewShowsThinkingSpinner(t *testing.T) {
	m := newTestModel()
	m.streamingState = StateStreaming

	view := m.View()

	if !containsString(view, "Thinking...") {
		t.Error("expected 'Thinking...' spinner when no content yet")
	}
}

func TestViewShowsFullReasoningDuringStreaming(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.streamingState = StateStreaming
	m.partialReasoning = "Let me analyze the code carefully"

	view := m.View()

	if !containsString(view, "Thinking...") {
		t.Error("expected 'Thinking...' spinner")
	}
	if !containsString(view, "Let me analyze the code carefully") {
		t.Error("expected full reasoning text in view")
	}
}

func TestReasoningTokenAccumulation(t *testing.T) {
	m := newTestModel()
	m.streamGeneration = 1
	m.streamingState = StateStreaming
	ch := make(chan llmclient.StreamMsg, 3)
	m.streamChan = ch

	m = updateModel(m, StreamTickMsg{
		Generation: 1,
		Event:      llmclient.StreamMsg{Type: llmclient.StreamMsgReasoningToken, Token: "Let me "},
	})
	m = updateModel(m, StreamTickMsg{
		Generation: 1,
		Event:      llmclient.StreamMsg{Type: llmclient.StreamMsgReasoningToken, Token: "think"},
	})

	if m.partialReasoning != "Let me think" {
		t.Errorf("expected 'Let me think', got %q", m.partialReasoning)
	}
	if m.partialContent != "" {
		t.Error("expected partialContent to remain empty during reasoning")
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

	if !containsString(view, "Echo") {
		t.Error("expected tool name in view")
	}
}

func TestViewRendersToolResults(t *testing.T) {
	m := newTestModel()
	// Push assistant tool call first, then result — result renders inline
	calls := []conversation.ToolCall{{
		ID:        "call_1",
		Name:      "echo",
		Arguments: json.RawMessage(`{"message":"hi"}`),
	}}
	m.conversation.Push(conversation.NewAssistantToolCallsMessage(calls))
	m.conversation.Push(conversation.NewToolResultMessage("call_1", "echo result"))

	view := m.View()

	if !containsString(view, "Result:") {
		t.Error("expected result label in view")
	}
	if !containsString(view, "echo result") {
		t.Error("expected tool result content in view")
	}
	if !containsString(view, "└─") {
		t.Error("expected L-connector in inline result")
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
	m.conversation.Push(conversation.NewUserMessage("first"))                  // idx 0
	m.conversation.Push(conversation.NewAssistantMessage("second"))            // idx 1
	m.conversation.Push(conversation.NewToolResultMessage("call_x", "result")) // idx 2 (RoleTool — skipped)
	m.conversation.Push(conversation.NewUserMessage("third"))                  // idx 3

	// Switch to history — should land on idx 3 (skipping tool at idx 2)
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.selectedMessage == nil || *m.selectedMessage != 3 {
		t.Fatalf("expected selection at idx 3, got %v", m.selectedMessage)
	}

	// Navigate up — should skip idx 2 (tool) and land on idx 1
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

	// Navigate down — should skip idx 2 (tool) from idx 1
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyDown})
	if *m.selectedMessage != 1 {
		t.Errorf("expected selection at idx 1, got %d", *m.selectedMessage)
	}

	m = updateModel(m, tea.KeyMsg{Type: tea.KeyDown})
	if *m.selectedMessage != 3 {
		t.Errorf("expected selection at idx 3 (skipping tool at 2), got %d", *m.selectedMessage)
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

func TestParallelToolResults(t *testing.T) {
	m := newTestModel()
	m.streamGeneration = 1
	m.streamingState = StateToolsExecuting
	m.pendingToolCalls = map[string]bool{"call_a": true, "call_b": true}

	calls := []conversation.ToolCall{
		{ID: "call_a", Name: "echo", Arguments: json.RawMessage(`{"msg":"a"}`)},
		{ID: "call_b", Name: "echo", Arguments: json.RawMessage(`{"msg":"b"}`)},
	}
	m.conversation.Push(conversation.NewAssistantToolCallsMessage(calls))

	// First result arrives — should stay in StateToolsExecuting
	m = updateModel(m, ToolResultMsg{Generation: 1, ToolCallID: "call_a", Content: "result a"})
	if m.streamingState != StateToolsExecuting {
		t.Errorf("expected StateToolsExecuting after first result, got %d", m.streamingState)
	}
	if len(m.pendingToolCalls) != 1 {
		t.Errorf("expected 1 pending call, got %d", len(m.pendingToolCalls))
	}

	// Second result arrives — should transition to StateStreaming
	m = updateModel(m, ToolResultMsg{Generation: 1, ToolCallID: "call_b", Content: "result b"})
	if m.streamingState != StateStreaming {
		t.Errorf("expected StateStreaming after all results, got %d", m.streamingState)
	}
	if len(m.pendingToolCalls) != 0 {
		t.Errorf("expected 0 pending calls, got %d", len(m.pendingToolCalls))
	}
}

func TestToolCallAccumulation(t *testing.T) {
	m := newTestModel()
	m.streamGeneration = 1
	m.streamingState = StateStreaming
	ch := make(chan llmclient.StreamMsg, 3)
	m.streamChan = ch

	call1 := conversation.ToolCall{ID: "call_1", Name: "echo", Arguments: json.RawMessage(`{}`)}
	call2 := conversation.ToolCall{ID: "call_2", Name: "echo", Arguments: json.RawMessage(`{}`)}

	// First tool call complete — should accumulate, not push to conversation
	m = updateModel(m, StreamTickMsg{
		Generation: 1,
		Event:      llmclient.StreamMsg{Type: llmclient.StreamMsgToolCallComplete, ToolCall: &call1},
	})
	if m.conversation.Len() != 0 {
		t.Error("expected no messages pushed during accumulation")
	}
	if len(m.accumulatedToolCalls) != 1 {
		t.Errorf("expected 1 accumulated call, got %d", len(m.accumulatedToolCalls))
	}

	// Second tool call complete
	m = updateModel(m, StreamTickMsg{
		Generation: 1,
		Event:      llmclient.StreamMsg{Type: llmclient.StreamMsgToolCallComplete, ToolCall: &call2},
	})
	if len(m.accumulatedToolCalls) != 2 {
		t.Errorf("expected 2 accumulated calls, got %d", len(m.accumulatedToolCalls))
	}
}

func TestToolErrorPushedAsResult(t *testing.T) {
	m := newTestModel()
	m.streamGeneration = 1
	m.streamingState = StateToolsExecuting
	m.pendingToolCalls = map[string]bool{"call_err": true}

	calls := []conversation.ToolCall{
		{ID: "call_err", Name: "bash_exec", Arguments: json.RawMessage(`{"command":"fail"}`)},
	}
	m.conversation.Push(conversation.NewAssistantToolCallsMessage(calls))

	// Tool error should become a result, not set StateError
	m = updateModel(m, ToolResultMsg{Generation: 1, ToolCallID: "call_err", Error: "command failed"})

	if m.streamingState == StateError {
		t.Error("tool error should not set StateError")
	}

	// Should have pushed a tool result message
	found := false
	for _, msg := range m.conversation.Messages {
		if msg.Role == conversation.RoleTool && msg.ToolResult != nil && msg.ToolResult.ToolCallID == "call_err" {
			if msg.ToolResult.Content != "Error: command failed" {
				t.Errorf("expected error content, got %q", msg.ToolResult.Content)
			}
			found = true
		}
	}
	if !found {
		t.Error("expected tool result message for error")
	}
}

func TestReasoningCollapsesOnFirstContentToken(t *testing.T) {
	m := newTestModel()
	m.streamGeneration = 1
	m.streamingState = StateStreaming
	ch := make(chan llmclient.StreamMsg, 3)
	m.streamChan = ch

	// Send reasoning tokens
	reasoningEvent := llmclient.StreamMsg{Type: llmclient.StreamMsgReasoningToken, Token: "Step 1\nStep 2"} //nolint:gosec // not credentials
	m = updateModel(m, StreamTickMsg{
		Generation: 1,
		Event:      reasoningEvent,
	})

	if m.reasoningCollapsed {
		t.Error("expected reasoningCollapsed to be false before content")
	}

	// First content token should trigger collapse
	m = updateModel(m, StreamTickMsg{
		Generation: 1,
		Event:      llmclient.StreamMsg{Type: llmclient.StreamMsgToken, Token: "Hello"},
	})

	if !m.reasoningCollapsed {
		t.Error("expected reasoningCollapsed to be true after first content token")
	}

	view := m.View()
	if !containsString(view, "Thinking (") {
		t.Error("expected collapsed reasoning summary in view")
	}
}

func TestReasoningPersistedOnFinalize(t *testing.T) {
	m := newTestModel()
	m.streamGeneration = 1
	m.streamingState = StateStreaming
	m.partialReasoning = "My reasoning here"
	m.partialContent = "The answer is 42"
	ch := make(chan llmclient.StreamMsg, 1)
	m.streamChan = ch

	m = updateModel(m, StreamTickMsg{
		Generation: 1,
		Event:      llmclient.StreamMsg{Type: llmclient.StreamMsgDone},
	})

	if m.conversation.Len() != 1 {
		t.Fatalf("expected 1 message, got %d", m.conversation.Len())
	}
	msg := m.conversation.Messages[0]
	if msg.Reasoning != "My reasoning here" {
		t.Errorf("expected reasoning to be persisted, got %q", msg.Reasoning)
	}
	if msg.Content != "The answer is 42" {
		t.Errorf("expected content 'The answer is 42', got %q", msg.Content)
	}
}

func TestReasoningAttachesToToolCallsMessage(t *testing.T) {
	m := newTestModel()
	m.streamGeneration = 1
	m.streamingState = StateStreaming
	ch := make(chan llmclient.StreamMsg, 3)
	m.streamChan = ch

	// Reasoning tokens
	reasoningEvent := llmclient.StreamMsg{Type: llmclient.StreamMsgReasoningToken, Token: "I should use a tool"} //nolint:gosec // not credentials
	m = updateModel(m, StreamTickMsg{
		Generation: 1,
		Event:      reasoningEvent,
	})

	// Tool call
	call := conversation.ToolCall{ID: "call_1", Name: "echo", Arguments: json.RawMessage(`{}`)}
	m = updateModel(m, StreamTickMsg{
		Generation: 1,
		Event:      llmclient.StreamMsg{Type: llmclient.StreamMsgToolCallComplete, ToolCall: &call},
	})

	// Done — no content, only tool calls
	m.pendingToolCalls = make(map[string]bool) // prevent actual tool exec
	m = updateModel(m, StreamTickMsg{
		Generation: 1,
		Event:      llmclient.StreamMsg{Type: llmclient.StreamMsgDone},
	})

	// Find the assistant tool calls message
	found := false
	for _, msg := range m.conversation.Messages {
		if msg.Role == conversation.RoleAssistant && len(msg.ToolCalls) > 0 {
			if msg.Reasoning != "I should use a tool" {
				t.Errorf("expected reasoning on tool calls message, got %q", msg.Reasoning)
			}
			found = true
		}
	}
	if !found {
		t.Error("expected assistant tool calls message")
	}
}

func TestReasoningExpandCollapseInHistory(t *testing.T) {
	m := newTestModel()
	msg := conversation.NewAssistantMessage("The answer is 42")
	msg.Reasoning = "Let me think\nabout this"
	m.conversation.Push(msg)

	msgID := m.conversation.Messages[0].Metadata.ID

	// View should show collapsed reasoning by default
	view := m.View()
	if !containsString(view, "Thinking (2 lines)") {
		t.Error("expected collapsed reasoning summary with line count")
	}

	// Switch to history, select the message
	m.focus = FocusHistory
	idx := 0
	m.selectedMessage = &idx

	// Expand
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.expandedBlocks[msgID] {
		t.Error("expected block to be expanded")
	}

	view = m.View()
	if !containsString(view, "Let me think") {
		t.Error("expected expanded reasoning text")
	}
	if !containsString(view, "about this") {
		t.Error("expected full reasoning text when expanded")
	}

	// Collapse
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.expandedBlocks[msgID] {
		t.Error("expected block to be collapsed")
	}
}
