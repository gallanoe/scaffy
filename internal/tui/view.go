package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/gallanoe/scaffy/internal/conversation"
)

// humanBytes formats a byte count as a human-readable string.
func humanBytes(b int) string {
	switch {
	case b >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(b)/(1024*1024))
	case b >= 1024:
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	messageHistory := m.renderMessageHistory()
	input := m.renderInputArea()
	statusBar := m.renderStatusBar()

	return lipgloss.JoinVertical(lipgloss.Left, messageHistory, input, statusBar)
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
		// Tool result messages are rendered inline with their parent tool call
		if msg.Role == conversation.RoleTool {
			continue
		}

		isSelected := m.focus == FocusHistory && m.selectedMessage != nil && *m.selectedMessage == idx
		expanded := m.expandedBlocks[msg.Metadata.ID]

		switch msg.Role {
		case conversation.RoleSystem:
			lines = append(lines, m.renderSystemMsg(msg, isSelected))
		case conversation.RoleUser:
			lines = append(lines, m.renderUserMsg(msg, isSelected))
		case conversation.RoleAssistant:
			lines = append(lines, m.renderAssistantMsg(msg, isSelected, expanded, cw)...)
		}

		lines = append(lines, "")
	}

	// Tool execution spinner
	if m.streamingState == StateToolsExecuting {
		n := len(m.pendingToolCalls)
		label := fmt.Sprintf(" Executing %d tool(s)...", n)
		spinnerLine := m.spinner.View() + m.styles.Text.Muted.Render(label)
		spinnerLine = m.addBorderPerLine(spinnerLine, m.styles.Message.AssistantBorder)
		lines = append(lines, spinnerLine)
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

func (m Model) renderSystemMsg(msg conversation.ChatMessage, isSelected bool) string {
	line := m.styles.Message.SystemText.Render(fmt.Sprintf("System: %s", msg.Content))
	if isSelected {
		line = m.styles.Message.SelectedBg.Render(line)
	}
	return line
}

func (m Model) renderUserMsg(msg conversation.ChatMessage, isSelected bool) string {
	border := m.styles.Message.UserBorderBlurred
	if isSelected {
		border = m.styles.Message.UserBorderFocused
	}
	rendered := m.addBorderPerLine(msg.Content, border)
	if isSelected {
		rendered = m.styles.Message.SelectedBg.Render(rendered)
	}
	return rendered
}

func (m Model) renderAssistantMsg(msg conversation.ChatMessage, isSelected, expanded bool, cw int) []string {
	var lines []string
	if len(msg.ToolCalls) > 0 {
		for _, call := range msg.ToolCalls {
			icon := m.toolStatusIcon(call.ID)
			display := m.formatToolCall(call, expanded)
			line := m.addBorderPerLine(icon+display, m.styles.Message.AssistantBorder)
			if isSelected {
				line = m.styles.Message.SelectedBg.Render(line)
			}
			lines = append(lines, line)

			// Inline result with L-connector
			if result := m.findToolResult(call.ID); result != nil {
				resultDisplay := m.formatToolResult(*result, expanded)
				resultLine := m.addBorderPerLine("   └─ "+resultDisplay, m.styles.Message.AssistantBorder)
				if isSelected {
					resultLine = m.styles.Message.SelectedBg.Render(resultLine)
				}
				lines = append(lines, resultLine)
			}
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
	return lines
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
	if r := m.findToolResult(callID); r != nil {
		if strings.HasPrefix(r.Content, "Error: ") {
			return m.styles.Tool.ErrorIcon
		}
		return m.styles.Tool.SuccessIcon
	}
	// Still pending (streaming or awaiting result)
	return m.styles.Tool.PendingIcon
}

// findToolResult scans the conversation for a tool result matching the given call ID.
func (m Model) findToolResult(callID string) *conversation.ToolResult {
	for _, msg := range m.conversation.Messages {
		if msg.Role == conversation.RoleTool && msg.ToolResult != nil && msg.ToolResult.ToolCallID == callID {
			return msg.ToolResult
		}
	}
	return nil
}

func (m Model) formatToolCall(call conversation.ToolCall, expanded bool) string {
	name := m.styles.Tool.Name.Render(snakeToCamel(call.Name))
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
	cutoff := 80
	if nl := strings.IndexByte(content, '\n'); nl >= 0 && nl < cutoff {
		cutoff = nl
	}
	if len(content) > cutoff {
		lineCount := strings.Count(content, "\n") + 1
		sizeStr := humanBytes(len(content))
		summary := m.styles.Text.HalfMuted.Render(fmt.Sprintf(" (%d lines, %s)", lineCount, sizeStr))
		return label + m.styles.Tool.ResultContent.Render(content[:cutoff]+"...") + summary
	}
	return label + m.styles.Tool.ResultContent.Render(content)
}

func snakeToCamel(s string) string {
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "")
}
