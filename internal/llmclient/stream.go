package llmclient

import (
	"encoding/json"

	openai "github.com/sashabaranov/go-openai"

	"github.com/gallanoe/scaffy/internal/conversation"
)

// StreamMsgType identifies the type of stream message.
type StreamMsgType int

const (
	StreamMsgToken StreamMsgType = iota
	StreamMsgToolCallStart
	StreamMsgToolCallArgDelta
	StreamMsgToolCallComplete
	StreamMsgDone
	StreamMsgError
)

// StreamMsg is a message from the LLM stream.
type StreamMsg struct {
	Type       StreamMsgType
	Token      string
	ToolCall   *conversation.ToolCall
	Index      int
	ID         string
	Name       string
	ArgDelta   string
	StopReason string
	Error      string
}

// PartialToolCall holds a tool call being assembled from streamed chunks.
type PartialToolCall struct {
	ID        string
	Name      string
	Arguments string
}

// ToolCallAccumulator assembles tool calls from streamed deltas.
type ToolCallAccumulator struct {
	Calls map[int]*PartialToolCall
}

func NewToolCallAccumulator() *ToolCallAccumulator {
	return &ToolCallAccumulator{
		Calls: make(map[int]*PartialToolCall),
	}
}

// ProcessDelta processes a tool call chunk from the stream. Returns a StreamMsg
// event and true if there's something to emit.
func (a *ToolCallAccumulator) ProcessDelta(chunk openai.ToolCall) (StreamMsg, bool) {
	idx := *chunk.Index

	// New tool call start (has ID)
	if chunk.ID != "" {
		name := ""
		if chunk.Function.Name != "" {
			name = chunk.Function.Name
		}
		a.Calls[idx] = &PartialToolCall{
			ID:   chunk.ID,
			Name: name,
		}
		return StreamMsg{
			Type:  StreamMsgToolCallStart,
			Index: idx,
			ID:    chunk.ID,
			Name:  name,
		}, true
	}

	// Argument delta
	if chunk.Function.Arguments != "" {
		if partial, ok := a.Calls[idx]; ok {
			partial.Arguments += chunk.Function.Arguments
			return StreamMsg{
				Type:     StreamMsgToolCallArgDelta,
				Index:    idx,
				ArgDelta: chunk.Function.Arguments,
			}, true
		}
	}

	return StreamMsg{}, false
}

// Finalize returns all accumulated tool calls as completed ToolCall values.
func (a *ToolCallAccumulator) Finalize() []conversation.ToolCall {
	calls := make([]conversation.ToolCall, 0, len(a.Calls))
	for _, p := range a.Calls {
		args := json.RawMessage(p.Arguments)
		if !json.Valid(args) {
			quoted, _ := json.Marshal(p.Arguments)
			args = quoted
		}
		calls = append(calls, conversation.ToolCall{
			ID:        p.ID,
			Name:      p.Name,
			Arguments: args,
		})
	}
	return calls
}
