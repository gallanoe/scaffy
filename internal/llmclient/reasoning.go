package llmclient

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// httpDoer matches go-openai's HTTPDoer interface.
type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// ReasoningConfig holds reasoning parameters for the OpenRouter API.
type ReasoningConfig struct {
	Effort    string
	MaxTokens int
	Exclude   bool
}

type reasoningDoer struct {
	base      httpDoer
	reasoning ReasoningConfig
}

// NewReasoningDoer wraps an httpDoer to inject reasoning config into chat completion requests.
func NewReasoningDoer(base httpDoer, cfg ReasoningConfig) httpDoer {
	return &reasoningDoer{base: base, reasoning: cfg}
}

func (d *reasoningDoer) Do(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodPost || !strings.HasSuffix(req.URL.Path, "/chat/completions") {
		return d.base.Do(req)
	}

	bodyBytes, err := io.ReadAll(req.Body)
	_ = req.Body.Close()
	if err != nil {
		return nil, err
	}

	var body map[string]any
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		return d.base.Do(req)
	}

	model, _ := body["model"].(string)
	reasoning := buildReasoningObj(model, d.reasoning)
	if len(reasoning) > 0 {
		body["reasoning"] = reasoning
	}

	newBytes, err := json.Marshal(body)
	if err != nil {
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		return d.base.Do(req)
	}

	req.Body = io.NopCloser(bytes.NewReader(newBytes))
	req.ContentLength = int64(len(newBytes))
	req.Header.Set("Content-Length", strconv.Itoa(len(newBytes)))

	resp, err := d.base.Do(req)
	if err != nil {
		return resp, err
	}

	// Wrap response body to transform reasoning_details -> reasoning_content
	// in SSE events so go-openai can parse reasoning tokens.
	resp.Body = &reasoningSSEReader{
		inner:  resp.Body,
		reader: bufio.NewReader(resp.Body),
	}
	return resp, nil
}

// buildReasoningObj returns the reasoning object fields appropriate for the model family.
func buildReasoningObj(model string, cfg ReasoningConfig) map[string]any {
	obj := make(map[string]any)

	effortOnly := isEffortOnlyModel(model)
	budgetBased := isBudgetBasedModel(model)

	switch {
	case effortOnly:
		if cfg.Effort != "" {
			obj["effort"] = cfg.Effort
		}
	case budgetBased:
		if cfg.MaxTokens > 0 {
			obj["max_tokens"] = cfg.MaxTokens
		} else if cfg.Effort != "" {
			obj["effort"] = cfg.Effort
		}
	default:
		// Unknown model: send effort if set, else max_tokens
		if cfg.Effort != "" {
			obj["effort"] = cfg.Effort
		} else if cfg.MaxTokens > 0 {
			obj["max_tokens"] = cfg.MaxTokens
		}
	}

	if cfg.Exclude {
		obj["exclude"] = true
	}

	return obj
}

func isEffortOnlyModel(model string) bool {
	return strings.HasPrefix(model, "openai/") || strings.HasPrefix(model, "x-ai/")
}

func isBudgetBasedModel(model string) bool {
	prefixes := []string{"anthropic/", "google/", "qwen/", "deepseek/"}
	for _, p := range prefixes {
		if strings.HasPrefix(model, p) {
			return true
		}
	}
	return false
}

// reasoningSSEReader wraps an SSE response body to transform OpenRouter's
// reasoning_details array into reasoning_content that go-openai can parse.
type reasoningSSEReader struct {
	inner  io.ReadCloser
	reader *bufio.Reader
	buf    bytes.Buffer
}

func (r *reasoningSSEReader) Read(p []byte) (int, error) {
	// Drain buffered output first
	if r.buf.Len() > 0 {
		return r.buf.Read(p)
	}

	line, err := r.reader.ReadBytes('\n')
	if len(line) == 0 {
		return 0, err
	}

	trimmed := bytes.TrimSpace(line)
	if bytes.HasPrefix(trimmed, []byte("data: ")) && !bytes.Equal(trimmed, []byte("data: [DONE]")) {
		jsonData := trimmed[len("data: "):]
		if transformed := transformReasoningDetails(jsonData); transformed != nil {
			r.buf.WriteString("data: ")
			r.buf.Write(transformed)
			r.buf.WriteByte('\n')
			n, _ := r.buf.Read(p)
			return n, err
		}
	}

	// Pass through unchanged
	r.buf.Write(line)
	n, _ := r.buf.Read(p)
	return n, err
}

func (r *reasoningSSEReader) Close() error {
	return r.inner.Close()
}

// transformReasoningDetails extracts text from reasoning_details in each
// choice's delta and sets it as reasoning_content. Returns nil if no
// transformation was needed.
func transformReasoningDetails(data []byte) []byte {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil
	}

	choices, ok := obj["choices"].([]any)
	if !ok || len(choices) == 0 {
		return nil
	}

	modified := false
	for _, c := range choices {
		if applyReasoningToDelta(c) {
			modified = true
		}
	}

	if !modified {
		return nil
	}

	result, err := json.Marshal(obj)
	if err != nil {
		return nil
	}
	return result
}

// applyReasoningToDelta extracts reasoning_details text from a single choice
// and sets reasoning_content on its delta. Returns true if modified.
func applyReasoningToDelta(c any) bool {
	choice, ok := c.(map[string]any)
	if !ok {
		return false
	}
	delta, ok := choice["delta"].(map[string]any)
	if !ok {
		return false
	}
	details, ok := delta["reasoning_details"].([]any)
	if !ok || len(details) == 0 {
		return false
	}

	var texts []string
	for _, d := range details {
		detail, ok := d.(map[string]any)
		if !ok {
			continue
		}
		if text, ok := detail["text"].(string); ok && text != "" {
			texts = append(texts, text)
		}
	}

	if len(texts) == 0 {
		return false
	}
	delta["reasoning_content"] = strings.Join(texts, "")
	return true
}
