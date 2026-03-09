use ratatui::{
    layout::{Constraint, Direction, Layout},
    style::{Color, Modifier, Style},
    text::{Line, Span},
    widgets::{Block, Borders, Paragraph, Wrap},
    Frame,
};

use crate::app::{App, AppFocus, StreamingState};
use crate::conversation::Role;

pub fn render(frame: &mut Frame, app: &App) {
    let has_error = matches!(app.streaming, StreamingState::Error(_));

    let mut constraints = vec![
        Constraint::Length(1),  // Status bar
        Constraint::Min(1),    // Message history
        Constraint::Length(5), // Input area
    ];
    if has_error {
        constraints.push(Constraint::Length(1)); // Error bar
    }

    let chunks = Layout::default()
        .direction(Direction::Vertical)
        .constraints(constraints)
        .split(frame.area());

    render_status_bar(frame, app, chunks[0]);
    render_message_history(frame, app, chunks[1]);
    render_input_area(frame, app, chunks[2]);

    if has_error {
        render_error_bar(frame, app, chunks[3]);
    }
}

fn render_status_bar(frame: &mut Frame, app: &App, area: ratatui::layout::Rect) {
    let model = &app
        .conversation
        .last_assistant_message()
        .and_then(|m| m.metadata.model.as_deref())
        .unwrap_or("no model");

    let streaming_indicator = match &app.streaming {
        StreamingState::Idle => "",
        StreamingState::Streaming { .. } => " streaming...",
        StreamingState::Error(_) => " error",
    };

    let status = Line::from(vec![
        Span::styled(
            format!(" Model: {} ", model),
            Style::default()
                .fg(Color::White)
                .bg(Color::DarkGray)
                .add_modifier(Modifier::BOLD),
        ),
        Span::styled(
            streaming_indicator.to_string(),
            Style::default()
                .fg(Color::Yellow)
                .bg(Color::DarkGray),
        ),
    ]);

    let bar = Paragraph::new(status).style(Style::default().bg(Color::DarkGray));
    frame.render_widget(bar, area);
}

fn render_message_history(frame: &mut Frame, app: &App, area: ratatui::layout::Rect) {
    let mut lines: Vec<Line> = Vec::new();

    for (idx, msg) in app.conversation.messages.iter().enumerate() {
        let is_selected = app.focus == AppFocus::History && app.selected_message == Some(idx);
        let expanded = app.expanded_tool_blocks.contains(&msg.metadata.id);

        match &msg.role {
            Role::System => {
                lines.push(Line::from(Span::styled(
                    format!("System: {}", msg.content.as_deref().unwrap_or("")),
                    Style::default()
                        .fg(Color::DarkGray)
                        .add_modifier(Modifier::DIM),
                )));
            }
            Role::User => {
                lines.push(Line::from(vec![
                    Span::styled(
                        "You: ",
                        Style::default()
                            .fg(Color::White)
                            .add_modifier(Modifier::BOLD),
                    ),
                    Span::raw(msg.content.as_deref().unwrap_or("")),
                ]));
            }
            Role::Assistant => {
                if let Some(tool_calls) = &msg.tool_calls {
                    for call in tool_calls {
                        let display = if expanded {
                            format!(
                                "[Tool: {}({})]",
                                call.name,
                                serde_json::to_string_pretty(&call.arguments)
                                    .unwrap_or_default()
                            )
                        } else {
                            let args_str = call.arguments.to_string();
                            let truncated = if args_str.len() > 60 {
                                format!("{}...", &args_str[..60])
                            } else {
                                args_str
                            };
                            format!("[Tool: {}({})]", call.name, truncated)
                        };
                        let style = if is_selected {
                            Style::default().fg(Color::Yellow).bg(Color::DarkGray)
                        } else {
                            Style::default().fg(Color::Yellow)
                        };
                        lines.push(Line::from(Span::styled(display, style)));
                    }
                } else {
                    lines.push(Line::from(vec![
                        Span::styled(
                            "Assistant: ",
                            Style::default()
                                .fg(Color::Green)
                                .add_modifier(Modifier::BOLD),
                        ),
                        Span::raw(msg.content.as_deref().unwrap_or("")),
                    ]));
                }
            }
            Role::Tool => {
                if let Some(result) = &msg.tool_result {
                    let display = if expanded {
                        format!("[Result: {}]", result.content)
                    } else {
                        let truncated = if result.content.len() > 80 {
                            format!("{}...", &result.content[..80])
                        } else {
                            result.content.clone()
                        };
                        format!("[Result: {}]", truncated)
                    };
                    let style = if is_selected {
                        Style::default().fg(Color::Cyan).bg(Color::DarkGray)
                    } else {
                        Style::default().fg(Color::Cyan)
                    };
                    lines.push(Line::from(Span::styled(display, style)));
                }
            }
        }

        // Highlight selected message
        if is_selected && msg.tool_calls.is_none() && msg.tool_result.is_none() {
            if let Some(last) = lines.last_mut() {
                *last = last.clone().style(Style::default().bg(Color::DarkGray));
            }
        }

        lines.push(Line::from(""));
    }

    // Append streaming content
    if let StreamingState::Streaming { partial_content } = &app.streaming {
        if !partial_content.is_empty() {
            lines.push(Line::from(vec![
                Span::styled(
                    "Assistant: ",
                    Style::default()
                        .fg(Color::Green)
                        .add_modifier(Modifier::BOLD),
                ),
                Span::raw(partial_content.as_str()),
                Span::styled("▋", Style::default().fg(Color::Green)),
            ]));
        } else {
            lines.push(Line::from(vec![
                Span::styled(
                    "Assistant: ",
                    Style::default()
                        .fg(Color::Green)
                        .add_modifier(Modifier::BOLD),
                ),
                Span::styled("▋", Style::default().fg(Color::Green)),
            ]));
        }
    }

    // Calculate scroll: auto-scroll to bottom unless user is browsing history
    let content_height = lines.len() as u16;
    let view_height = area.height.saturating_sub(2); // account for borders
    let scroll = if app.focus == AppFocus::History {
        app.scroll_offset
    } else {
        content_height.saturating_sub(view_height)
    };

    let history = Paragraph::new(lines)
        .block(
            Block::default()
                .borders(Borders::ALL)
                .title(" Messages "),
        )
        .wrap(Wrap { trim: false })
        .scroll((scroll, 0));

    frame.render_widget(history, area);
}

fn render_input_area(frame: &mut Frame, app: &App, area: ratatui::layout::Rect) {
    let border_style = if app.focus == AppFocus::Input {
        Style::default().fg(Color::Cyan)
    } else {
        Style::default().fg(Color::DarkGray)
    };

    let block = Block::default()
        .borders(Borders::ALL)
        .title(" Input (Enter to send, Shift+Enter for newline) ")
        .border_style(border_style);

    frame.render_widget(block, area);

    let inner = area.inner(ratatui::layout::Margin {
        vertical: 1,
        horizontal: 1,
    });
    frame.render_widget(&app.textarea, inner);
}

fn render_error_bar(frame: &mut Frame, app: &App, area: ratatui::layout::Rect) {
    if let StreamingState::Error(ref msg) = app.streaming {
        let error = Paragraph::new(Line::from(vec![
            Span::styled(
                " Error: ",
                Style::default()
                    .fg(Color::White)
                    .bg(Color::Red)
                    .add_modifier(Modifier::BOLD),
            ),
            Span::styled(
                msg.as_str(),
                Style::default().fg(Color::White).bg(Color::Red),
            ),
            Span::styled(
                " (Esc to dismiss)",
                Style::default()
                    .fg(Color::LightRed)
                    .bg(Color::Red),
            ),
        ]))
        .style(Style::default().bg(Color::Red));
        frame.render_widget(error, area);
    }
}
