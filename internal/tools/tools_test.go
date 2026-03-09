package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gallanoe/scaffy/internal/conversation"
)

func TestEchoToolExecute(t *testing.T) {
	tool := &EchoTool{}
	args := json.RawMessage(`{"message":"hello"}`)
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if parsed["message"] != "hello" {
		t.Errorf("expected message 'hello', got %v", parsed["message"])
	}
}

func TestRegistryRegisterAndGet(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&EchoTool{})

	h, ok := reg.Get("echo")
	if !ok {
		t.Fatal("expected to find echo tool")
	}
	if h.Name() != "echo" {
		t.Errorf("expected name 'echo', got %q", h.Name())
	}

	_, ok = reg.Get("nonexistent")
	if ok {
		t.Error("expected not to find nonexistent tool")
	}
}

func TestRegistryExecute(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&EchoTool{})

	call := &conversation.ToolCall{
		ID:        "1",
		Name:      "echo",
		Arguments: json.RawMessage(`{"message":"test"}`),
	}
	result, err := reg.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if parsed["message"] != "test" {
		t.Errorf("expected message 'test', got %v", parsed["message"])
	}
}

func TestRegistryToOpenAITools(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&EchoTool{})

	tools := reg.ToOpenAITools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Function.Name != "echo" {
		t.Errorf("expected function name 'echo', got %q", tools[0].Function.Name)
	}
}

func TestRegistryExecuteUnknownTool(t *testing.T) {
	reg := NewRegistry()

	call := &conversation.ToolCall{
		ID:        "1",
		Name:      "unknown",
		Arguments: json.RawMessage(`{}`),
	}
	_, err := reg.Execute(context.Background(), call)
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestRegistryIsEmpty(t *testing.T) {
	reg := NewRegistry()
	if !reg.IsEmpty() {
		t.Error("expected empty registry")
	}
	reg.Register(&EchoTool{})
	if reg.IsEmpty() {
		t.Error("expected non-empty registry")
	}
}
