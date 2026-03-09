package markdown

import (
	"strings"
	"testing"
)

func TestNewRenderer(t *testing.T) {
	r := NewRenderer(80)
	if r == nil {
		t.Fatal("NewRenderer returned nil")
	}
}

func TestRendererProducesOutput(t *testing.T) {
	r := NewRenderer(80)
	out, err := r.Render("# Hello\n\nWorld")
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if !strings.Contains(out, "Hello") {
		t.Error("expected rendered output to contain 'Hello'")
	}
	if !strings.Contains(out, "World") {
		t.Error("expected rendered output to contain 'World'")
	}
}

func TestCacheHit(t *testing.T) {
	c := NewCache()

	first := c.GetOrRender("**bold**", 80)
	second := c.GetOrRender("**bold**", 80)

	if first != second {
		t.Error("expected cache hit to return identical result")
	}
}

func TestCacheDifferentWidth(t *testing.T) {
	c := NewCache()

	a := c.GetOrRender("test", 40)
	b := c.GetOrRender("test", 80)

	// Both should render, different keys
	if a == "" || b == "" {
		t.Error("expected non-empty renders")
	}
}
