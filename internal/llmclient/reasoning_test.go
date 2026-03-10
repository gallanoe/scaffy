package llmclient

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// fakeDoer records the request for inspection.
type fakeDoer struct {
	lastReq *http.Request
}

func (f *fakeDoer) Do(req *http.Request) (*http.Response, error) {
	f.lastReq = req
	return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(nil))}, nil
}

func readBody(t *testing.T, req *http.Request) map[string]any {
	t.Helper()
	b, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func makeReq(t *testing.T, method, path string, body map[string]any) *http.Request {
	t.Helper()
	b, _ := json.Marshal(body)
	req, err := http.NewRequest(method, "https://openrouter.ai/api/v1"+path, bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestReasoningDoer_EffortOnlyModel(t *testing.T) {
	fake := &fakeDoer{}
	doer := NewReasoningDoer(fake, ReasoningConfig{Effort: "high", MaxTokens: 4096})

	req := makeReq(t, "POST", "/chat/completions", map[string]any{
		"model":    "openai/o3",
		"messages": []any{},
	})
	if _, err := doer.Do(req); err != nil {
		t.Fatal(err)
	}

	body := readBody(t, fake.lastReq)
	reasoning, ok := body["reasoning"].(map[string]any)
	if !ok {
		t.Fatal("expected reasoning object in body")
	}
	if reasoning["effort"] != "high" {
		t.Errorf("expected effort 'high', got %v", reasoning["effort"])
	}
	if _, exists := reasoning["max_tokens"]; exists {
		t.Error("effort-only model should not have max_tokens")
	}
}

func TestReasoningDoer_BudgetModelWithMaxTokens(t *testing.T) {
	fake := &fakeDoer{}
	doer := NewReasoningDoer(fake, ReasoningConfig{Effort: "high", MaxTokens: 4096})

	req := makeReq(t, "POST", "/chat/completions", map[string]any{
		"model":    "anthropic/claude-sonnet-4",
		"messages": []any{},
	})
	if _, err := doer.Do(req); err != nil {
		t.Fatal(err)
	}

	body := readBody(t, fake.lastReq)
	reasoning, ok := body["reasoning"].(map[string]any)
	if !ok {
		t.Fatal("expected reasoning object in body")
	}
	if reasoning["max_tokens"] != float64(4096) {
		t.Errorf("expected max_tokens 4096, got %v", reasoning["max_tokens"])
	}
	if _, exists := reasoning["effort"]; exists {
		t.Error("budget model with max_tokens should not have effort")
	}
}

func TestReasoningDoer_BudgetModelEffortFallback(t *testing.T) {
	fake := &fakeDoer{}
	doer := NewReasoningDoer(fake, ReasoningConfig{Effort: "high"})

	req := makeReq(t, "POST", "/chat/completions", map[string]any{
		"model":    "anthropic/claude-sonnet-4",
		"messages": []any{},
	})
	if _, err := doer.Do(req); err != nil {
		t.Fatal(err)
	}

	body := readBody(t, fake.lastReq)
	reasoning, ok := body["reasoning"].(map[string]any)
	if !ok {
		t.Fatal("expected reasoning object in body")
	}
	if reasoning["effort"] != "high" {
		t.Errorf("expected effort fallback 'high', got %v", reasoning["effort"])
	}
	if _, exists := reasoning["max_tokens"]; exists {
		t.Error("should not have max_tokens when only effort set")
	}
}

func TestReasoningDoer_NonMatchingRequest(t *testing.T) {
	fake := &fakeDoer{}
	doer := NewReasoningDoer(fake, ReasoningConfig{Effort: "high"})

	// GET request — should pass through unchanged
	req := makeReq(t, "GET", "/models", map[string]any{})
	if _, err := doer.Do(req); err != nil {
		t.Fatal(err)
	}

	// POST to different path — should pass through unchanged
	req = makeReq(t, "POST", "/embeddings", map[string]any{
		"model": "openai/o3",
		"input": "test",
	})
	if _, err := doer.Do(req); err != nil {
		t.Fatal(err)
	}

	body := readBody(t, fake.lastReq)
	if _, exists := body["reasoning"]; exists {
		t.Error("non-matching request should not have reasoning injected")
	}
}

func TestReasoningDoer_ExcludeFlag(t *testing.T) {
	fake := &fakeDoer{}
	doer := NewReasoningDoer(fake, ReasoningConfig{Effort: "high", Exclude: true})

	req := makeReq(t, "POST", "/chat/completions", map[string]any{
		"model":    "openai/o3",
		"messages": []any{},
	})
	if _, err := doer.Do(req); err != nil {
		t.Fatal(err)
	}

	body := readBody(t, fake.lastReq)
	reasoning := body["reasoning"].(map[string]any)
	if reasoning["exclude"] != true {
		t.Errorf("expected exclude true, got %v", reasoning["exclude"])
	}
}

func TestReasoningDoer_ExcludeFalseOmitted(t *testing.T) {
	fake := &fakeDoer{}
	doer := NewReasoningDoer(fake, ReasoningConfig{Effort: "high", Exclude: false})

	req := makeReq(t, "POST", "/chat/completions", map[string]any{
		"model":    "openai/o3",
		"messages": []any{},
	})
	if _, err := doer.Do(req); err != nil {
		t.Fatal(err)
	}

	body := readBody(t, fake.lastReq)
	reasoning := body["reasoning"].(map[string]any)
	if _, exists := reasoning["exclude"]; exists {
		t.Error("exclude: false should be omitted")
	}
}

func TestTransformReasoningDetails(t *testing.T) {
	// SSE event with reasoning_details should get reasoning_content added
	input := `{"choices":[{"delta":{"reasoning_details":[{"type":"reasoning.text","text":"Let me think..."}]}}]}`
	result := transformReasoningDetails([]byte(input))
	if result == nil {
		t.Fatal("expected transformation, got nil")
	}
	var obj map[string]any
	if err := json.Unmarshal(result, &obj); err != nil {
		t.Fatal(err)
	}
	choices := obj["choices"].([]any)
	delta := choices[0].(map[string]any)["delta"].(map[string]any)
	if delta["reasoning_content"] != "Let me think..." {
		t.Errorf("expected reasoning_content 'Let me think...', got %v", delta["reasoning_content"])
	}
}

func TestTransformReasoningDetailsNoDetails(t *testing.T) {
	// SSE event without reasoning_details should return nil (no transform)
	input := `{"choices":[{"delta":{"content":"Hello"}}]}`
	result := transformReasoningDetails([]byte(input))
	if result != nil {
		t.Error("expected nil for event without reasoning_details")
	}
}

func TestTransformReasoningDetailsMultipleTexts(t *testing.T) {
	input := `{"choices":[{"delta":{"reasoning_details":[{"type":"reasoning.text","text":"A"},{"type":"reasoning.text","text":"B"}]}}]}`
	result := transformReasoningDetails([]byte(input))
	if result == nil {
		t.Fatal("expected transformation")
	}
	var obj map[string]any
	if err := json.Unmarshal(result, &obj); err != nil {
		t.Fatal(err)
	}
	delta := obj["choices"].([]any)[0].(map[string]any)["delta"].(map[string]any)
	if delta["reasoning_content"] != "AB" {
		t.Errorf("expected concatenated 'AB', got %v", delta["reasoning_content"])
	}
}

func TestReasoningSSEReader(t *testing.T) {
	// Simulate an SSE stream with reasoning_details
	sseData := "data: {\"choices\":[{\"delta\":{\"reasoning_details\":[{\"type\":\"reasoning.text\",\"text\":\"thinking\"}]}}]}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\ndata: [DONE]\n\n"
	body := io.NopCloser(strings.NewReader(sseData))
	reader := &reasoningSSEReader{
		inner:  body,
		reader: bufio.NewReader(body),
	}
	defer func() { _ = reader.Close() }()

	all, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	output := string(all)

	if !strings.Contains(output, "reasoning_content") {
		t.Error("expected reasoning_content in transformed output")
	}
	if !strings.Contains(output, "thinking") {
		t.Error("expected reasoning text preserved in output")
	}
	if !strings.Contains(output, "[DONE]") {
		t.Error("expected [DONE] sentinel preserved")
	}
	if !strings.Contains(output, `"content":"hello"`) {
		t.Error("expected content event preserved")
	}
}

func TestBuildReasoningObj(t *testing.T) {
	tests := []struct {
		name      string
		model     string
		cfg       ReasoningConfig
		wantKeys  []string
		noKeys    []string
		wantEmpty bool
	}{
		{
			name:     "openai effort only",
			model:    "openai/o3-mini",
			cfg:      ReasoningConfig{Effort: "medium", MaxTokens: 1024},
			wantKeys: []string{"effort"},
			noKeys:   []string{"max_tokens"},
		},
		{
			name:     "x-ai effort only",
			model:    "x-ai/grok-3-mini-beta",
			cfg:      ReasoningConfig{Effort: "low"},
			wantKeys: []string{"effort"},
		},
		{
			name:     "anthropic with max_tokens",
			model:    "anthropic/claude-sonnet-4",
			cfg:      ReasoningConfig{MaxTokens: 2048},
			wantKeys: []string{"max_tokens"},
			noKeys:   []string{"effort"},
		},
		{
			name:     "google with effort fallback",
			model:    "google/gemini-2.5-pro",
			cfg:      ReasoningConfig{Effort: "high"},
			wantKeys: []string{"effort"},
		},
		{
			name:     "qwen budget based",
			model:    "qwen/qwq-32b",
			cfg:      ReasoningConfig{MaxTokens: 8192},
			wantKeys: []string{"max_tokens"},
		},
		{
			name:     "deepseek budget based",
			model:    "deepseek/deepseek-r1",
			cfg:      ReasoningConfig{MaxTokens: 4096, Effort: "high"},
			wantKeys: []string{"max_tokens"},
			noKeys:   []string{"effort"},
		},
		{
			name:     "unknown model with effort",
			model:    "custom/my-model",
			cfg:      ReasoningConfig{Effort: "high"},
			wantKeys: []string{"effort"},
		},
		{
			name:     "unknown model with max_tokens only",
			model:    "custom/my-model",
			cfg:      ReasoningConfig{MaxTokens: 1024},
			wantKeys: []string{"max_tokens"},
		},
		{
			name:      "empty config",
			model:     "openai/o3",
			cfg:       ReasoningConfig{},
			wantEmpty: true,
		},
		{
			name:     "exclude always included when true",
			model:    "anthropic/claude-sonnet-4",
			cfg:      ReasoningConfig{Effort: "high", Exclude: true},
			wantKeys: []string{"effort", "exclude"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := buildReasoningObj(tt.model, tt.cfg)
			if tt.wantEmpty {
				if len(obj) != 0 {
					t.Errorf("expected empty object, got %v", obj)
				}
				return
			}
			for _, k := range tt.wantKeys {
				if _, ok := obj[k]; !ok {
					t.Errorf("expected key %q in %v", k, obj)
				}
			}
			for _, k := range tt.noKeys {
				if _, ok := obj[k]; ok {
					t.Errorf("unexpected key %q in %v", k, obj)
				}
			}
		})
	}
}
