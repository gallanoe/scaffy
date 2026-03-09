package conversation

import (
	"encoding/json"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

func TestToOpenAIMessages(t *testing.T) {
	conv := NewConversation()
	conv.Push(NewSystemMessage("sys"))
	conv.Push(NewUserMessage("usr"))
	conv.Push(NewAssistantMessage("asst"))
	conv.Push(NewToolResultMessage("tc1", "result"))

	msgs := conv.ToOpenAIMessages()
	if len(msgs) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(msgs))
	}
	if msgs[0].Role != openai.ChatMessageRoleSystem {
		t.Errorf("expected system role, got %s", msgs[0].Role)
	}
	if msgs[1].Role != openai.ChatMessageRoleUser {
		t.Errorf("expected user role, got %s", msgs[1].Role)
	}
	if msgs[2].Role != openai.ChatMessageRoleAssistant {
		t.Errorf("expected assistant role, got %s", msgs[2].Role)
	}
	if msgs[3].Role != openai.ChatMessageRoleTool {
		t.Errorf("expected tool role, got %s", msgs[3].Role)
	}
	if msgs[3].ToolCallID != "tc1" {
		t.Errorf("expected tool call ID 'tc1', got %q", msgs[3].ToolCallID)
	}
}

func TestToOpenAIMessagesWithToolCalls(t *testing.T) {
	conv := NewConversation()
	calls := []ToolCall{{
		ID:        "call_1",
		Name:      "echo",
		Arguments: json.RawMessage(`{"message":"hi"}`),
	}}
	conv.Push(NewAssistantToolCallsMessage(calls))

	msgs := conv.ToOpenAIMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if len(msgs[0].ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(msgs[0].ToolCalls))
	}
	tc := msgs[0].ToolCalls[0]
	if tc.ID != "call_1" {
		t.Errorf("expected tool call ID 'call_1', got %q", tc.ID)
	}
	if tc.Function.Name != "echo" {
		t.Errorf("expected function name 'echo', got %q", tc.Function.Name)
	}
	if tc.Function.Arguments != `{"message":"hi"}` {
		t.Errorf("unexpected arguments: %s", tc.Function.Arguments)
	}
}

func TestPushFromResponse(t *testing.T) {
	conv := NewConversation()
	resp := openai.ChatCompletionResponse{
		Model: "test-model",
		Choices: []openai.ChatCompletionChoice{
			{
				Message: openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleAssistant,
					Content: "Hello there!",
				},
			},
		},
		Usage: openai.Usage{CompletionTokens: 42},
	}

	conv.PushFromResponse(resp)
	if conv.Len() != 1 {
		t.Fatalf("expected 1 message, got %d", conv.Len())
	}
	msg := conv.Messages[0]
	if msg.Content != "Hello there!" {
		t.Errorf("expected content 'Hello there!', got %q", msg.Content)
	}
	if msg.Metadata.Model != "test-model" {
		t.Errorf("expected model 'test-model', got %q", msg.Metadata.Model)
	}
	if msg.Metadata.TokenCount != 42 {
		t.Errorf("expected token count 42, got %d", msg.Metadata.TokenCount)
	}
}

func TestPushFromResponseWithToolCalls(t *testing.T) {
	conv := NewConversation()
	resp := openai.ChatCompletionResponse{
		Model: "test-model",
		Choices: []openai.ChatCompletionChoice{
			{
				Message: openai.ChatCompletionMessage{
					Role: openai.ChatMessageRoleAssistant,
					ToolCalls: []openai.ToolCall{
						{
							ID:   "call_abc",
							Type: openai.ToolTypeFunction,
							Function: openai.FunctionCall{
								Name:      "echo",
								Arguments: `{"message":"hello"}`,
							},
						},
					},
				},
			},
		},
		Usage: openai.Usage{CompletionTokens: 10},
	}

	conv.PushFromResponse(resp)
	if conv.Len() != 1 {
		t.Fatalf("expected 1 message, got %d", conv.Len())
	}
	msg := conv.Messages[0]
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(msg.ToolCalls))
	}
	if msg.ToolCalls[0].ID != "call_abc" {
		t.Errorf("expected tool call ID 'call_abc', got %q", msg.ToolCalls[0].ID)
	}
	if msg.ToolCalls[0].Name != "echo" {
		t.Errorf("expected tool name 'echo', got %q", msg.ToolCalls[0].Name)
	}
}

func TestPushFromResponseEmpty(t *testing.T) {
	conv := NewConversation()
	resp := openai.ChatCompletionResponse{}
	conv.PushFromResponse(resp)
	if conv.Len() != 0 {
		t.Error("expected no messages for empty response")
	}
}
