package conversation

import (
	"encoding/json"

	openai "github.com/sashabaranov/go-openai"
)

// ToOpenAIMessages converts internal messages to go-openai request messages.
func (c *Conversation) ToOpenAIMessages() []openai.ChatCompletionMessage {
	msgs := make([]openai.ChatCompletionMessage, 0, len(c.Messages))

	for _, msg := range c.Messages {
		switch msg.Role {
		case RoleSystem:
			msgs = append(msgs, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleSystem,
				Content: msg.Content,
			})
		case RoleUser:
			msgs = append(msgs, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: msg.Content,
			})
		case RoleAssistant:
			m := openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleAssistant,
				Content: msg.Content,
			}
			if len(msg.ToolCalls) > 0 {
				m.ToolCalls = make([]openai.ToolCall, len(msg.ToolCalls))
				for i, tc := range msg.ToolCalls {
					m.ToolCalls[i] = openai.ToolCall{
						ID:   tc.ID,
						Type: openai.ToolTypeFunction,
						Function: openai.FunctionCall{
							Name:      tc.Name,
							Arguments: string(tc.Arguments),
						},
					}
				}
			}
			msgs = append(msgs, m)
		case RoleTool:
			if msg.ToolResult != nil {
				msgs = append(msgs, openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					Content:    msg.ToolResult.Content,
					ToolCallID: msg.ToolResult.ToolCallID,
				})
			}
		}
	}

	return msgs
}

// PushFromResponse converts an OpenAI chat completion response into internal
// types and pushes to the conversation.
func (c *Conversation) PushFromResponse(resp openai.ChatCompletionResponse) {
	if len(resp.Choices) == 0 {
		return
	}

	choice := resp.Choices[0]
	respMsg := choice.Message

	if len(respMsg.ToolCalls) > 0 {
		calls := make([]ToolCall, len(respMsg.ToolCalls))
		for i, tc := range respMsg.ToolCalls {
			args := json.RawMessage(tc.Function.Arguments)
			// Validate JSON; if invalid, wrap as a JSON string
			if !json.Valid(args) {
				quoted, _ := json.Marshal(tc.Function.Arguments)
				args = quoted
			}
			calls[i] = ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: args,
			}
		}
		msg := NewAssistantToolCallsMessage(calls)
		msg.Metadata.Model = resp.Model
		if resp.Usage.CompletionTokens > 0 {
			msg.Metadata.TokenCount = resp.Usage.CompletionTokens
		}
		c.Push(msg)
		return
	}

	if respMsg.Content != "" {
		msg := NewAssistantMessage(respMsg.Content)
		msg.Metadata.Model = resp.Model
		if resp.Usage.CompletionTokens > 0 {
			msg.Metadata.TokenCount = resp.Usage.CompletionTokens
		}
		c.Push(msg)
	}
}
