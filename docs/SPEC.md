# Scaffy — Agent Scaffold Spec

## Overview

Scaffy is a TUI-based LLM agent scaffold built in Go. It provides a chat interface with streaming responses, tool calling, and an internal conversation model decoupled from any LLM provider's wire format. The initial backend is OpenRouter (via `go-openai`'s configurable base URL).

## Dependencies

| Purpose | Package |
|---------|---------|
| LLM API | `github.com/sashabaranov/go-openai` |
| TUI framework | `github.com/charmbracelet/bubbletea` (v1) |
| Text input | `github.com/charmbracelet/bubbles/textarea` |
| Scrollable area | `github.com/charmbracelet/bubbles/viewport` |
| Styling | `github.com/charmbracelet/lipgloss` |
| UUIDs | `github.com/google/uuid` |
| .env loading | `github.com/joho/godotenv` |
| JSON | `encoding/json` (stdlib) |
| Logging | `log/slog` (stdlib, to file) |

### Dependency Notes

- `go-openai` provides streaming, tool calling, and configurable base URL supporting OpenRouter and any OpenAI-compatible provider.
- Bubbletea v1 (stable) implements The Elm Architecture for TUI applications.
- No external time packages — timestamps use `time.Time` from stdlib.

## Project Structure

```
scaffy/
  cmd/scaffy/main.go                     -- entry point
  internal/
    config/config.go                      -- env var loading, Config struct
    config/config_test.go
    conversation/conversation.go          -- ChatMessage, Conversation, Role, ToolCall types
    conversation/conversation_test.go
    conversation/openai.go                -- ToOpenAIMessages(), PushFromResponse()
    conversation/openai_test.go
    tools/tools.go                        -- ToolHandler interface, ToolRegistry
    tools/echo.go                         -- EchoTool
    tools/tools_test.go
    llmclient/client.go                   -- LlmClient, config
    llmclient/stream.go                   -- StreamMsg types, ToolCallAccumulator
    llmclient/stream_test.go
    tui/model.go                          -- Bubbletea Model
    tui/update.go                         -- Update() handler (event dispatch)
    tui/view.go                           -- View() renderer
    tui/messages.go                       -- tea.Msg types
    tui/model_test.go                     -- TUI integration tests
  .golangci.yml
  .github/workflows/ci.yml
  .env.example
  .gitignore
  docs/SPEC.md
  go.mod
  go.sum
  Makefile
  LICENSE
```

---

## Internal Conversation Model (`internal/conversation/`)

Decoupled from `go-openai`'s types so we can:
- Add metadata (timestamps, IDs, token counts)
- Persist conversations without depending on a provider's types
- Transform messages before sending (truncation, summarization)
- Swap providers without touching the rest of the codebase

### Types

```go
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

type ChatMessage struct {
    Role       Role            `json:"role"`
    Content    string          `json:"content,omitempty"`
    ToolCalls  []ToolCall      `json:"tool_calls,omitempty"`
    ToolResult *ToolResult     `json:"tool_result,omitempty"`
    Metadata   MessageMetadata `json:"metadata"`
}
```

### Constructors

Type-safe constructors prevent invalid states:

```go
func NewSystemMessage(content string) ChatMessage
func NewUserMessage(content string) ChatMessage
func NewAssistantMessage(content string) ChatMessage
func NewAssistantToolCallsMessage(calls []ToolCall) ChatMessage
func NewToolResultMessage(toolCallID, content string) ChatMessage
```

### Conversation Methods

```go
func (c *Conversation) Push(msg ChatMessage)
func (c *Conversation) Len() int
func (c *Conversation) IsEmpty() bool
func (c *Conversation) LastAssistantMessage() *ChatMessage
func (c *Conversation) Clear()
func (c *Conversation) ToOpenAIMessages() []openai.ChatCompletionMessage
func (c *Conversation) PushFromResponse(resp openai.ChatCompletionResponse)
```

---

## Tool System (`internal/tools/`)

### ToolHandler Interface

```go
type ToolHandler interface {
    Name() string
    Description() string
    ParametersSchema() json.RawMessage
    Execute(ctx context.Context, args json.RawMessage) (string, error)
}
```

### ToolRegistry

```go
func NewRegistry() *ToolRegistry
func (r *ToolRegistry) Register(handler ToolHandler)
func (r *ToolRegistry) Get(name string) (ToolHandler, bool)
func (r *ToolRegistry) ToOpenAITools() []openai.Tool
func (r *ToolRegistry) Execute(ctx context.Context, call *ToolCall) (string, error)
func (r *ToolRegistry) IsEmpty() bool
```

---

## Stream Events (`internal/llmclient/`)

`go-openai` streams `ChatCompletionStreamResponse` items. We define our own `StreamMsg` type to decouple from the library:

```go
type StreamMsgType int

const (
    StreamMsgToken StreamMsgType = iota
    StreamMsgToolCallStart
    StreamMsgToolCallArgDelta
    StreamMsgToolCallComplete
    StreamMsgDone
    StreamMsgError
)

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
```

### Stream Parsing

The `ToolCallAccumulator` assembles streamed tool calls:

```go
type ToolCallAccumulator struct {
    Calls map[int]*PartialToolCall
}

func (a *ToolCallAccumulator) ProcessDelta(chunk openai.ToolCall) (StreamMsg, bool)
func (a *ToolCallAccumulator) Finalize() []conversation.ToolCall
```

---

## LLM Client (`internal/llmclient/`)

Wraps `go-openai` and exposes a clean interface.

```go
func NewLlmClient(apiKey, baseURL, model string, maxTokens int, temperature float64) *LlmClient
func (c *LlmClient) ChatStream(ctx context.Context, messages []openai.ChatCompletionMessage, tools []openai.Tool) <-chan StreamMsg
```

`ChatStream` starts a goroutine that sends `StreamMsg` values to the returned channel. The channel is closed when the stream ends. Cancel the context to abandon the stream.

---

## TUI (`internal/tui/`)

Uses The Elm Architecture (Bubbletea): `Init() -> Cmd`, `Update(Msg) -> (Model, Cmd)`, `View() -> string`.

### Layout

```
+-----------------------------------------------+
|                                                 |
|  System: You are a helpful assistant.           |
|                                                 |
|  What's the weather in Paris?                   |
|                                                 |
|  I'll check the weather for you.                |
|                                                 |
|  [Tool: get_weather({"city": "Paris"})]         |  <- Truncated, expandable
|  [Result: 15C, partly cloudy]                   |  <- Truncated, expandable
|                                                 |
|  The weather in Paris is 15°C and partly cloudy.|
|                                                 |
+-----------------------------------------------+
| ╭─────────────────────────────────────────────╮ |
| │ Type a message...                           │ |  <- textarea (multi-line)
| ╰─────────────────────────────────────────────╯ |
+-----------------------------------------------+
|  Error: Connection timed out (Esc to dismiss) |  <- Error bar (only when error)
+-----------------------------------------------+
```

### Panels

1. **Message history** (top, scrollable via viewport) — all messages rendered by role
2. **Input area** (bottom) — bubbles textarea with focus-aware border
3. **Error bar** (bottom, conditional) — shown only when an error occurs, dismissed with Esc

### Keybindings

| Key | Action |
|-----|--------|
| Enter | Send message (when input focused) |
| Alt+Enter | Newline in input |
| Ctrl+C | Quit |
| Up/Down | Navigate messages (when history focused) |
| Tab | Toggle focus between input and history |
| Enter (on tool block) | Expand/collapse tool call/result |
| Esc | Clear error / return focus to input |

### Streaming Pattern

LLM streaming runs in a goroutine that writes `StreamMsg` values to a Go channel. A `tea.Cmd` reads one event at a time from this channel and returns it as a `tea.Msg`. Each `Update()` call processes the event and returns another listener `Cmd` to read the next event:

```go
func waitForStream(ch <-chan StreamMsg, gen uint64) tea.Cmd {
    return func() tea.Msg {
        event, ok := <-ch
        if !ok { return StreamDoneMsg{Generation: gen} }
        return StreamTickMsg{Generation: gen, Event: event}
    }
}
```

Generation-based stale event filtering is preserved: each `StreamTickMsg` carries a generation, and `Update()` discards messages where `msg.Generation != m.streamGeneration`.

---

## Agent Loop (tool calling)

The agent loop is driven through Bubbletea's command system. The UI remains responsive throughout.

1. User sends message → increment generation → start stream
2. `StreamTickMsg` with `Token` → append to partial content
3. `StreamTickMsg` with `ToolCallComplete` → finalize partial content, push tool call message, execute tool as `tea.Cmd`
4. `ToolResultMsg` → push result, increment generation, start new stream
5. `StreamTickMsg` with `Done` → finalize assistant message, set idle

Tool execution runs as a `tea.Cmd` (goroutine managed by Bubbletea). Results flow back as `ToolResultMsg` values, tagged with the generation that initiated them.

---

## Configuration

Environment variables (loaded via `.env`):

```
OPENROUTER_API_KEY=sk-or-...
SCAFFY_MODEL=anthropic/claude-sonnet-4
SCAFFY_MAX_TOKENS=4096
SCAFFY_TEMPERATURE=0.7
SCAFFY_SYSTEM_PROMPT="You are a helpful assistant."
SCAFFY_ECHO_TOOL=1
```

---

## Building & Running

```bash
# Build
make build

# Run
make run

# Test
make test

# Lint
make lint
```

---

## Future Considerations (not implemented now)

- Context window management (truncation/summarization)
- Conversation persistence (save/load from disk)
- Side panel for conversation list
- Image/multi-modal support
- Multiple provider backends
- Custom tool registration via config file
- Parallel tool execution
