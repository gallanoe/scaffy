package tools

import (
	"context"
	"encoding/json"
	"fmt"

	openai "github.com/sashabaranov/go-openai"

	"github.com/gallanoe/scaffy/internal/conversation"
)

// ToolHandler defines the interface for executable tools.
type ToolHandler interface {
	Name() string
	Description() string
	ParametersSchema() json.RawMessage
	Execute(ctx context.Context, args json.RawMessage) (string, error)
}

// ToolRegistry manages registered tool handlers.
type ToolRegistry struct {
	handlers map[string]ToolHandler
}

func NewRegistry() *ToolRegistry {
	return &ToolRegistry{
		handlers: make(map[string]ToolHandler),
	}
}

func (r *ToolRegistry) Register(handler ToolHandler) {
	r.handlers[handler.Name()] = handler
}

func (r *ToolRegistry) Get(name string) (ToolHandler, bool) {
	h, ok := r.handlers[name]
	return h, ok
}

func (r *ToolRegistry) IsEmpty() bool {
	return len(r.handlers) == 0
}

func (r *ToolRegistry) ToOpenAITools() []openai.Tool {
	tools := make([]openai.Tool, 0, len(r.handlers))
	for _, h := range r.handlers {
		var params any
		_ = json.Unmarshal(h.ParametersSchema(), &params)
		tools = append(tools, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        h.Name(),
				Description: h.Description(),
				Parameters:  params,
			},
		})
	}
	return tools
}

func (r *ToolRegistry) Execute(ctx context.Context, call *conversation.ToolCall) (string, error) {
	h, ok := r.handlers[call.Name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", call.Name)
	}
	return h.Execute(ctx, call.Arguments)
}
