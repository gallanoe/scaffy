package styles

import "testing"

func TestNewStyles(t *testing.T) {
	s := NewStyles()
	if s == nil {
		t.Fatal("NewStyles() returned nil")
	}
	if s.MaxWidth != 120 {
		t.Errorf("expected MaxWidth 120, got %d", s.MaxWidth)
	}
}

func TestStylesSubStructsInitialized(t *testing.T) {
	s := NewStyles()

	// Spot-check that rendered border strings are non-empty.
	if s.Message.UserBorderFocused == "" {
		t.Error("UserBorderFocused is empty")
	}
	if s.Message.AssistantBorder == "" {
		t.Error("AssistantBorder is empty")
	}
	if s.Tool.PendingIcon == "" {
		t.Error("PendingIcon is empty")
	}
	if s.Tool.SuccessIcon == "" {
		t.Error("SuccessIcon is empty")
	}
	if s.Tool.ErrorIcon == "" {
		t.Error("ErrorIcon is empty")
	}
}
