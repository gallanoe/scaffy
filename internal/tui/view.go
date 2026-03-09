package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/gallanoe/scaffy/internal/conversation"
)

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	header := m.renderHeader()
	messageHistory := m.renderMessageHistory()
	input := m.renderInputArea()
	statusBar := m.renderStatusBar()

	return lipgloss.JoinVertical(lipgloss.Left, header, messageHistory, input, statusBar)
}

func (m Model) contentWidth() int {
	w := m.width - 4 // border + padding
	if w > m.styles.MaxWidth {
		w = m.styles.MaxWidth
	}
	if w < 20 {
		w = 20
	}
	return w
}

func (m Model) renderHeader() string {
	appName := m.styles.Header.AppName.Render("scaffy")
	sep := m.styles.Text.Subtle.Render(" · ")
	modelName := m.styles.Header.ModelName.Render(m.modelName)

	content := appName + sep + modelName
	return m.styles.Header.Bar.Width(m.width).Render(content)
}

func (m Model) renderStatusBar() string {
	if m.streamingState == StateError && m.errorMessage != "" {
		label := m.styles.StatusBar.ErrorLabel.Render(" Error: ")
		msg := m.styles.StatusBar.ErrorText.Render(m.errorMessage)
		hint := m.styles.Text.HalfMuted.Render(" (Esc to dismiss)")
		return m.styles.StatusBar.Base.Width(m.width).Render(label + msg + hint)
	}

	hints := []string{
		m.styles.StatusBar.HintKey.Render("Tab") + m.styles.StatusBar.HintDesc.Render(" focus"),
		m.styles.StatusBar.HintKey.Render("Enter") + m.styles.StatusBar.HintDesc.Render(" send"),
		m.styles.StatusBar.HintKey.Render("Alt+Enter") + m.styles.StatusBar.HintDesc.Render(" newline"),
		m.styles.StatusBar.HintKey.Render("Ctrl+C") + m.styles.StatusBar.HintDesc.Render(" quit"),
	}
	return m.styles.StatusBar.Base.Width(m.width).Render(strings.Join(hints, m.styles.Text.Subtle.Render(" · ")))
}

func (m Model) renderMessageHistory() string {
	var lines []string
	cw := m.contentWidth()

	for idx, msg := range m.conversation.Messages {
		isSelected := m.focus == FocusHistory && m.selectedMessage != nil && *m.selectedMessage == idx
		expanded := m.expandedBlocks[msg.Metadata.ID]

		switch msg.Role {
		case conversation.RoleSystem:
			line := m.styles.Message.SystemText.Render(fmt.Sprintf("System: %s", msg.Content))
			if isSelected {
				line = m.styles.Message.SelectedBg.Render(line)
			}
			lines = append(lines, line)

		case conversation.RoleUser:
			border := m.styles.Message.UserBorderBlurred
			if isSelected {
				border = m.styles.Message.UserBorderFocused
			}
			rendered := m.addBorderPerLine(msg.Content, border)
			if isSelected {
				rendered = m.styles.Message.SelectedBg.Render(rendered)
			}
			lines = append(lines, rendered)

		case conversation.RoleAssistant:
			if len(msg.ToolCalls) > 0 {
				for _, call := range msg.ToolCalls {
					icon := m.toolStatusIcon(call.ID)
					display := m.formatToolCall(call, expanded)
					line := m.addBorderPerLine(icon+display, m.styles.Message.AssistantBorder)
					if isSelected {
						line = m.styles.Message.SelectedBg.Render(line)
					}
					lines = append(lines, line)
				}
			} else {
				rendered := m.mdCache.GetOrRender(msg.Content, cw)
				rendered = strings.TrimRight(rendered, "\n")
				rendered = m.addBorderPerLine(rendered, m.styles.Message.AssistantBorder)
				if isSelected {
					rendered = m.styles.Message.SelectedBg.Render(rendered)
				}
				lines = append(lines, rendered)
			}

		case conversation.RoleTool:
			if msg.ToolResult != nil {
				display := m.formatToolResult(*msg.ToolResult, expanded)
				line := m.addBorderPerLine(display, m.styles.Message.AssistantBorder)
				if isSelected {
					line = m.styles.Message.SelectedBg.Render(line)
				}
				lines = append(lines, line)
			}
		}

		lines = append(lines, "")
	}

	// Streaming content + spinner
	if m.streamingState == StateStreaming {
		if m.partialContent != "" {
			rendered := m.mdCache.GetOrRender(m.partialContent, cw)
			rendered = strings.TrimRight(rendered, "\n")
			partial := m.addBorderPerLine(rendered, m.styles.Message.AssistantBorder)
			lines = append(lines, partial)
		}
		spinnerLine := m.spinner.View() + m.styles.Text.Muted.Render(" Thinking...")
		spinnerLine = m.addBorderPerLine(spinnerLine, m.styles.Message.AssistantBorder)
		lines = append(lines, spinnerLine)
	}

	content := strings.Join(lines, "\n")

	if m.viewportReady {
		m.viewport.SetContent(content)
		// Auto-scroll to bottom unless browsing history
		if m.focus != FocusHistory {
			m.viewport.GotoBottom()
		}
		return m.viewport.View()
	}

	return content
}

func (m Model) renderInputArea() string {
	var style lipgloss.Style
	if m.focus == FocusInput {
		style = m.styles.Input.Focused
	} else {
		style = m.styles.Input.Blurred
	}

	return style.
		Width(m.width - 2).
		Render(m.textarea.View())
}

func (m Model) addBorderPerLine(text, border string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = border + line
	}
	return strings.Join(lines, "\n")
}

// toolStatusIcon returns the appropriate status icon for a tool call by
// checking if a result exists for it in the conversation.
func (m Model) toolStatusIcon(callID string) string {
	for _, msg := range m.conversation.Messages {
		if msg.Role == conversation.RoleTool && msg.ToolResult != nil && msg.ToolResult.ToolCallID == callID {
			return m.styles.Tool.SuccessIcon
		}
	}
	// Still pending (streaming or awaiting result)
	return m.styles.Tool.PendingIcon
}

func (m Model) formatToolCall(call conversation.ToolCall, expanded bool) string {
	name := m.styles.Tool.Name.Render(call.Name)
	if expanded {
		var prettyArgs any
		if err := json.Unmarshal(call.Arguments, &prettyArgs); err == nil {
			pretty, _ := json.MarshalIndent(prettyArgs, "", "  ")
			return name + m.styles.Tool.Args.Render("("+string(pretty)+")")
		}
		return name + m.styles.Tool.Args.Render("("+string(call.Arguments)+")")
	}

	argsStr := string(call.Arguments)
	if len(argsStr) > 60 {
		argsStr = argsStr[:60] + "..."
	}
	return name + m.styles.Tool.Args.Render("("+argsStr+")")
}

func (m Model) formatToolResult(result conversation.ToolResult, expanded bool) string {
	label := m.styles.Tool.ResultLabel.Render("Result: ")
	if expanded {
		return label + m.styles.Tool.ResultContent.Render(result.Content)
	}
	content := result.Content
	if len(content) > 80 {
		content = content[:80] + "..."
	}
	return label + m.styles.Tool.ResultContent.Render(content)
}
