package llmclient

import (
	"context"
	"net/http"

	openai "github.com/sashabaranov/go-openai"
)

type LlmClient struct {
	client      *openai.Client
	Model       string
	MaxTokens   int
	Temperature float32
}

func NewLlmClient(apiKey, baseURL, model string, maxTokens int, temperature float64, reasoning *ReasoningConfig) *LlmClient {
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = baseURL

	if reasoning != nil {
		var base httpDoer = &http.Client{}
		config.HTTPClient = NewReasoningDoer(base, *reasoning)
	}

	return &LlmClient{
		client:      openai.NewClientWithConfig(config),
		Model:       model,
		MaxTokens:   maxTokens,
		Temperature: float32(temperature),
	}
}

// ChatStream starts a streaming chat completion in a goroutine.
// Events are sent to the returned channel. The channel is closed when the stream ends.
// Cancel the context to abandon the stream.
func (c *LlmClient) ChatStream(ctx context.Context, messages []openai.ChatCompletionMessage, tools []openai.Tool) <-chan StreamMsg {
	ch := make(chan StreamMsg, 100)

	go func() {
		defer close(ch)
		c.doChatStream(ctx, messages, tools, ch)
	}()

	return ch
}

func (c *LlmClient) doChatStream(ctx context.Context, messages []openai.ChatCompletionMessage, tools []openai.Tool, ch chan<- StreamMsg) {
	req := openai.ChatCompletionRequest{
		Model:               c.Model,
		Messages:            messages,
		MaxCompletionTokens: c.MaxTokens,
		Temperature:         c.Temperature,
		Stream:              true,
	}
	if len(tools) > 0 {
		req.Tools = tools
	}

	stream, err := c.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		ch <- StreamMsg{Type: StreamMsgError, Error: err.Error()}
		return
	}
	defer func() { _ = stream.Close() }()

	acc := NewToolCallAccumulator()

	for {
		resp, err := stream.Recv()
		if err != nil {
			// io.EOF means stream ended normally
			if err.Error() == "EOF" {
				ch <- StreamMsg{Type: StreamMsgDone}
				return
			}
			ch <- StreamMsg{Type: StreamMsgError, Error: err.Error()}
			return
		}

		if len(resp.Choices) == 0 {
			continue
		}
		choice := resp.Choices[0]

		// Reasoning tokens
		if choice.Delta.ReasoningContent != "" {
			ch <- StreamMsg{Type: StreamMsgReasoningToken, Token: choice.Delta.ReasoningContent}
		}

		// Content tokens
		if choice.Delta.Content != "" {
			ch <- StreamMsg{Type: StreamMsgToken, Token: choice.Delta.Content}
		}

		// Tool call deltas
		for _, tc := range choice.Delta.ToolCalls {
			if event, ok := acc.ProcessDelta(tc); ok {
				ch <- event
			}
		}

		// Finish reason
		if choice.FinishReason != "" {
			switch choice.FinishReason {
			case openai.FinishReasonToolCalls:
				for _, call := range acc.Finalize() {
					ch <- StreamMsg{Type: StreamMsgToolCallComplete, ToolCall: &call}
				}
				ch <- StreamMsg{Type: StreamMsgDone, StopReason: "tool_calls"}
			case openai.FinishReasonStop:
				ch <- StreamMsg{Type: StreamMsgDone, StopReason: "stop"}
			default:
				ch <- StreamMsg{Type: StreamMsgDone, StopReason: string(choice.FinishReason)}
			}
			return
		}
	}
}
