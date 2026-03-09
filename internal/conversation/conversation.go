package conversation

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Content    string `json:"content"`
}

type MessageMetadata struct {
	ID         uuid.UUID `json:"id"`
	Timestamp  time.Time `json:"timestamp"`
	Model      string    `json:"model,omitempty"`
	TokenCount int       `json:"token_count,omitempty"`
}

func newMetadata() MessageMetadata {
	return MessageMetadata{
		ID:        uuid.New(),
		Timestamp: time.Now(),
	}
}

type ChatMessage struct {
	Role       Role             `json:"role"`
	Content    string           `json:"content,omitempty"`
	ToolCalls  []ToolCall       `json:"tool_calls,omitempty"`
	ToolResult *ToolResult      `json:"tool_result,omitempty"`
	Metadata   MessageMetadata  `json:"metadata"`
}

func NewSystemMessage(content string) ChatMessage {
	return ChatMessage{
		Role:     RoleSystem,
		Content:  content,
		Metadata: newMetadata(),
	}
}

func NewUserMessage(content string) ChatMessage {
	return ChatMessage{
		Role:     RoleUser,
		Content:  content,
		Metadata: newMetadata(),
	}
}

func NewAssistantMessage(content string) ChatMessage {
	return ChatMessage{
		Role:     RoleAssistant,
		Content:  content,
		Metadata: newMetadata(),
	}
}

func NewAssistantToolCallsMessage(calls []ToolCall) ChatMessage {
	return ChatMessage{
		Role:      RoleAssistant,
		ToolCalls: calls,
		Metadata:  newMetadata(),
	}
}

func NewToolResultMessage(toolCallID, content string) ChatMessage {
	return ChatMessage{
		Role: RoleTool,
		ToolResult: &ToolResult{
			ToolCallID: toolCallID,
			Content:    content,
		},
		Metadata: newMetadata(),
	}
}

type Conversation struct {
	Messages []ChatMessage
}

func NewConversation() *Conversation {
	return &Conversation{}
}

func (c *Conversation) Push(msg ChatMessage) {
	c.Messages = append(c.Messages, msg)
}

func (c *Conversation) Len() int {
	return len(c.Messages)
}

func (c *Conversation) IsEmpty() bool {
	return len(c.Messages) == 0
}

func (c *Conversation) LastAssistantMessage() *ChatMessage {
	for i := len(c.Messages) - 1; i >= 0; i-- {
		if c.Messages[i].Role == RoleAssistant {
			return &c.Messages[i]
		}
	}
	return nil
}

func (c *Conversation) Clear() {
	c.Messages = nil
}
