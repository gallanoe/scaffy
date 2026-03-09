package styles

import "github.com/charmbracelet/lipgloss"

// Color palette (hex true-color).
var (
	Primary   = lipgloss.Color("#a78bfa")
	Secondary = lipgloss.Color("#f472b6")
	Tertiary  = lipgloss.Color("#4ade80")

	BgBase    = lipgloss.Color("#1e1e2e")
	BgLighter = lipgloss.Color("#313244")
	BgSubtle  = lipgloss.Color("#45475a")
	BgOverlay = lipgloss.Color("#585b70")

	FgBase     = lipgloss.Color("#cdd6f4")
	FgMuted    = lipgloss.Color("#a6adc8")
	FgHalfMuted = lipgloss.Color("#7f849c")
	FgSubtle   = lipgloss.Color("#585b70")

	StatusError   = lipgloss.Color("#f38ba8")
	StatusWarning = lipgloss.Color("#f9e2af")
	StatusInfo    = lipgloss.Color("#89b4fa")
	StatusSuccess = lipgloss.Color("#a6e3a1")
)

type TextStyles struct {
	Base      lipgloss.Style
	Muted     lipgloss.Style
	HalfMuted lipgloss.Style
	Subtle    lipgloss.Style
}

type HeaderStyles struct {
	Bar       lipgloss.Style
	AppName   lipgloss.Style
	ModelName lipgloss.Style
}

type MessageStyles struct {
	UserBorderFocused string
	UserBorderBlurred string
	AssistantBorder   string
	SystemText        lipgloss.Style
	SelectedBg        lipgloss.Style
}

type ToolStyles struct {
	PendingIcon   string
	SuccessIcon   string
	ErrorIcon     string
	Name          lipgloss.Style
	Args          lipgloss.Style
	ResultLabel   lipgloss.Style
	ResultContent lipgloss.Style
}

type InputStyles struct {
	Focused lipgloss.Style
	Blurred lipgloss.Style
}

type StatusBarStyles struct {
	Base       lipgloss.Style
	ErrorLabel lipgloss.Style
	ErrorText  lipgloss.Style
	HintKey    lipgloss.Style
	HintDesc   lipgloss.Style
}

type Styles struct {
	Text      TextStyles
	Header    HeaderStyles
	Message   MessageStyles
	Tool      ToolStyles
	Input     InputStyles
	StatusBar StatusBarStyles
	Spinner   lipgloss.Style
	MaxWidth  int
}

func NewStyles() *Styles {
	return &Styles{
		Text: TextStyles{
			Base:      lipgloss.NewStyle().Foreground(FgBase),
			Muted:     lipgloss.NewStyle().Foreground(FgMuted),
			HalfMuted: lipgloss.NewStyle().Foreground(FgHalfMuted),
			Subtle:    lipgloss.NewStyle().Foreground(FgSubtle),
		},
		Header: HeaderStyles{
			Bar:       lipgloss.NewStyle().Background(BgLighter).Padding(0, 1),
			AppName:   lipgloss.NewStyle().Foreground(Primary).Bold(true),
			ModelName: lipgloss.NewStyle().Foreground(FgMuted),
		},
		Message: MessageStyles{
			UserBorderFocused: lipgloss.NewStyle().Foreground(Primary).Render("▌ "),
			UserBorderBlurred: lipgloss.NewStyle().Foreground(Primary).Render("│ "),
			AssistantBorder:   lipgloss.NewStyle().Foreground(Tertiary).Render("│ "),
			SystemText:        lipgloss.NewStyle().Foreground(FgMuted).Faint(true),
			SelectedBg:        lipgloss.NewStyle().Background(BgSubtle),
		},
		Tool: ToolStyles{
			PendingIcon:   lipgloss.NewStyle().Foreground(StatusWarning).Render("● "),
			SuccessIcon:   lipgloss.NewStyle().Foreground(StatusSuccess).Render("✓ "),
			ErrorIcon:     lipgloss.NewStyle().Foreground(StatusError).Render("✗ "),
			Name:          lipgloss.NewStyle().Bold(true).Foreground(FgBase),
			Args:          lipgloss.NewStyle().Foreground(FgMuted),
			ResultLabel:   lipgloss.NewStyle().Foreground(StatusInfo),
			ResultContent: lipgloss.NewStyle().Foreground(FgMuted),
		},
		Input: InputStyles{
			Focused: lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(Primary).
				Padding(0, 1),
			Blurred: lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(FgSubtle).
				Padding(0, 1),
		},
		StatusBar: StatusBarStyles{
			Base:       lipgloss.NewStyle().Background(BgLighter).Padding(0, 1),
			ErrorLabel: lipgloss.NewStyle().Foreground(StatusError).Bold(true),
			ErrorText:  lipgloss.NewStyle().Foreground(StatusError),
			HintKey:    lipgloss.NewStyle().Foreground(FgMuted).Bold(true),
			HintDesc:   lipgloss.NewStyle().Foreground(FgHalfMuted),
		},
		Spinner:  lipgloss.NewStyle().Foreground(Primary),
		MaxWidth: 120,
	}
}
