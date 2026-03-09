package conversation

import (
	"encoding/json"
	"testing"
)

func TestConstructors(t *testing.T) {
	msg := NewSystemMessage("test")
	if msg.Role != RoleSystem {
		t.Errorf("expected System role, got %s", msg.Role)
	}
	if msg.Content != "test" {
		t.Errorf("expected content 'test', got %q", msg.Content)
	}

	msg = NewUserMessage("hello")
	if msg.Role != RoleUser {
		t.Errorf("expected User role, got %s", msg.Role)
	}

	msg = NewAssistantMessage("response")
	if msg.Role != RoleAssistant {
		t.Errorf("expected Assistant role, got %s", msg.Role)
	}
	if len(msg.ToolCalls) != 0 {
		t.Error("expected no tool calls")
	}

	calls := []ToolCall{{
		ID:        "1",
		Name:      "echo",
		Arguments: json.RawMessage(`{"text":"hi"}`),
	}}
	msg = NewAssistantToolCallsMessage(calls)
	if len(msg.ToolCalls) != 1 {
		t.Error("expected 1 tool call")
	}
	if msg.Content != "" {
		t.Error("expected no content for tool calls message")
	}

	msg = NewToolResultMessage("1", "result")
	if msg.Role != RoleTool {
		t.Errorf("expected Tool role, got %s", msg.Role)
	}
	if msg.ToolResult == nil || msg.ToolResult.ToolCallID != "1" {
		t.Error("expected tool result with call ID '1'")
	}
}

func TestConversationPushAndLen(t *testing.T) {
	conv := NewConversation()
	if !conv.IsEmpty() {
		t.Error("expected empty conversation")
	}

	conv.Push(NewUserMessage("hi"))
	conv.Push(NewAssistantMessage("hello"))
	if conv.Len() != 2 {
		t.Errorf("expected len 2, got %d", conv.Len())
	}

	last := conv.LastAssistantMessage()
	if last == nil || last.Content != "hello" {
		t.Error("expected last assistant message 'hello'")
	}
}

func TestIsEmpty(t *testing.T) {
	conv := NewConversation()
	if !conv.IsEmpty() {
		t.Error("new conversation should be empty")
	}
	conv.Push(NewUserMessage("x"))
	if conv.IsEmpty() {
		t.Error("conversation with message should not be empty")
	}
}

func TestLastAssistantMessage(t *testing.T) {
	conv := NewConversation()
	if conv.LastAssistantMessage() != nil {
		t.Error("expected nil for empty conversation")
	}

	conv.Push(NewUserMessage("hi"))
	if conv.LastAssistantMessage() != nil {
		t.Error("expected nil when no assistant messages")
	}

	conv.Push(NewAssistantMessage("first"))
	conv.Push(NewUserMessage("another"))
	conv.Push(NewAssistantMessage("second"))

	last := conv.LastAssistantMessage()
	if last == nil || last.Content != "second" {
		t.Error("expected last assistant message 'second'")
	}
}

func TestClear(t *testing.T) {
	conv := NewConversation()
	conv.Push(NewUserMessage("hi"))
	conv.Clear()
	if !conv.IsEmpty() {
		t.Error("expected empty after clear")
	}
}
