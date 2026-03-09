package llmclient

import (
	"encoding/json"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

func intPtr(i int) *int { return &i }

func TestAccumulatorStart(t *testing.T) {
	acc := NewToolCallAccumulator()

	chunk := openai.ToolCall{
		Index: intPtr(0),
		ID:    "call_123",
		Type:  openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name: "echo",
		},
	}

	event, ok := acc.ProcessDelta(chunk)
	if !ok {
		t.Fatal("expected event from start chunk")
	}
	if event.Type != StreamMsgToolCallStart {
		t.Errorf("expected ToolCallStart, got %d", event.Type)
	}
	if event.ID != "call_123" {
		t.Errorf("expected ID 'call_123', got %q", event.ID)
	}
	if event.Name != "echo" {
		t.Errorf("expected name 'echo', got %q", event.Name)
	}
}

func TestAccumulatorDelta(t *testing.T) {
	acc := NewToolCallAccumulator()

	// Start
	acc.ProcessDelta(openai.ToolCall{
		Index: intPtr(0),
		ID:    "call_123",
		Function: openai.FunctionCall{
			Name: "echo",
		},
	})

	// Arg delta
	chunk := openai.ToolCall{
		Index: intPtr(0),
		Function: openai.FunctionCall{
			Arguments: `{"message": `,
		},
	}

	event, ok := acc.ProcessDelta(chunk)
	if !ok {
		t.Fatal("expected event from delta chunk")
	}
	if event.Type != StreamMsgToolCallArgDelta {
		t.Errorf("expected ToolCallArgDelta, got %d", event.Type)
	}
	if event.ArgDelta != `{"message": ` {
		t.Errorf("unexpected arg delta: %q", event.ArgDelta)
	}
}

func TestAccumulatorFinalize(t *testing.T) {
	acc := NewToolCallAccumulator()

	// Start
	acc.ProcessDelta(openai.ToolCall{
		Index: intPtr(0),
		ID:    "call_123",
		Function: openai.FunctionCall{
			Name: "echo",
		},
	})

	// Arg deltas
	acc.ProcessDelta(openai.ToolCall{
		Index: intPtr(0),
		Function: openai.FunctionCall{
			Arguments: `{"message": `,
		},
	})
	acc.ProcessDelta(openai.ToolCall{
		Index: intPtr(0),
		Function: openai.FunctionCall{
			Arguments: `"hello"}`,
		},
	})

	calls := acc.Finalize()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].ID != "call_123" {
		t.Errorf("expected ID 'call_123', got %q", calls[0].ID)
	}
	if calls[0].Name != "echo" {
		t.Errorf("expected name 'echo', got %q", calls[0].Name)
	}

	var args map[string]string
	if err := json.Unmarshal(calls[0].Arguments, &args); err != nil {
		t.Fatalf("failed to unmarshal arguments: %v", err)
	}
	if args["message"] != "hello" {
		t.Errorf("expected message 'hello', got %q", args["message"])
	}
}

func TestAccumulatorMultipleCalls(t *testing.T) {
	acc := NewToolCallAccumulator()

	// Start first call
	acc.ProcessDelta(openai.ToolCall{
		Index: intPtr(0),
		ID:    "call_1",
		Function: openai.FunctionCall{
			Name: "echo",
		},
	})

	// Start second call
	acc.ProcessDelta(openai.ToolCall{
		Index: intPtr(1),
		ID:    "call_2",
		Function: openai.FunctionCall{
			Name: "echo",
		},
	})

	// Args for first
	acc.ProcessDelta(openai.ToolCall{
		Index: intPtr(0),
		Function: openai.FunctionCall{
			Arguments: `{"message":"one"}`,
		},
	})

	// Args for second
	acc.ProcessDelta(openai.ToolCall{
		Index: intPtr(1),
		Function: openai.FunctionCall{
			Arguments: `{"message":"two"}`,
		},
	})

	calls := acc.Finalize()
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(calls))
	}

	// Verify both calls present (order not guaranteed from map)
	ids := map[string]bool{}
	for _, c := range calls {
		ids[c.ID] = true
	}
	if !ids["call_1"] || !ids["call_2"] {
		t.Error("expected both call_1 and call_2")
	}
}
