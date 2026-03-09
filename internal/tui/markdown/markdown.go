package markdown

import (
	"sync"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/lipgloss"
)

// Palette colors used in the custom style.
var (
	primary   = "#a78bfa"
	secondary = "#f472b6"
	info      = "#89b4fa"
	fgBase    = "#cdd6f4"
	fgMuted   = "#a6adc8"
	bgSubtle  = "#45475a"
)

func boolPtr(b bool) *bool    { return &b }
func strPtr(s string) *string { return &s }
func uintPtr(u uint) *uint    { return &u }

func customStyle() ansi.StyleConfig {
	return ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: &fgBase,
			},
			Margin: uintPtr(0),
		},
		Heading: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Bold:  boolPtr(true),
				Color: &primary,
			},
		},
		H1: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Bold:  boolPtr(true),
				Color: &primary,
			},
		},
		H2: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Bold:  boolPtr(true),
				Color: &primary,
			},
		},
		H3: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Bold:  boolPtr(true),
				Color: &primary,
			},
		},
		BlockQuote: ansi.StyleBlock{
			Indent:      uintPtr(1),
			IndentToken: strPtr("│ "),
			StylePrimitive: ansi.StylePrimitive{
				Color:  &fgMuted,
				Italic: boolPtr(true),
			},
		},
		List: ansi.StyleList{
			LevelIndent: 2,
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color: &fgBase,
				},
			},
		},
		Item: ansi.StylePrimitive{
			BlockPrefix: "• ",
			Color:       &fgBase,
		},
		Link: ansi.StylePrimitive{
			Color:     &info,
			Underline: boolPtr(true),
		},
		LinkText: ansi.StylePrimitive{
			Color: &info,
			Bold:  boolPtr(true),
		},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:           &secondary,
				BackgroundColor: &bgSubtle,
			},
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				Margin: uintPtr(1),
				StylePrimitive: ansi.StylePrimitive{
					Color: &fgBase,
				},
			},
			Chroma: &ansi.Chroma{
				Text: ansi.StylePrimitive{Color: &fgBase},
			},
		},
		Emph: ansi.StylePrimitive{
			Italic: boolPtr(true),
			Color:  &fgBase,
		},
		Strong: ansi.StylePrimitive{
			Bold:  boolPtr(true),
			Color: &fgBase,
		},
		Table: ansi.StyleTable{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color: &fgBase,
				},
			},
		},
	}
}

// NewRenderer creates a glamour renderer with our custom palette.
func NewRenderer(width int) *glamour.TermRenderer {
	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(customStyle()),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		// Fall back to plain dark style on error.
		r, _ = glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(width),
		)
	}
	return r
}

type cacheKey struct {
	content string
	width   int
}

// Cache avoids re-rendering markdown on every View() call.
type Cache struct {
	mu      sync.Mutex
	entries map[cacheKey]string
}

func NewCache() *Cache {
	return &Cache{entries: make(map[cacheKey]string)}
}

// GetOrRender returns cached rendered markdown, or renders and caches it.
func (c *Cache) GetOrRender(content string, width int) string {
	key := cacheKey{content: content, width: width}

	c.mu.Lock()
	if v, ok := c.entries[key]; ok {
		c.mu.Unlock()
		return v
	}
	c.mu.Unlock()

	r := NewRenderer(width)
	rendered, err := r.Render(content)
	if err != nil {
		rendered = content
	}
	// Glamour adds trailing newlines; trim for inline display.
	rendered = lipgloss.NewStyle().Render(rendered)

	c.mu.Lock()
	c.entries[key] = rendered
	c.mu.Unlock()

	return rendered
}
