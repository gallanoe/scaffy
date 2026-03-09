use crossterm::event::{KeyCode, KeyEvent, KeyModifiers};
use std::collections::HashSet;
use std::sync::Arc;
use tokio::sync::mpsc;
use tui_textarea::{Input, Key, TextArea};
use uuid::Uuid;

use crate::conversation::{ChatMessage, Conversation, ToolCall};
use crate::llm_client::{LlmClient, StreamEvent};
use crate::tools::ToolRegistry;

#[derive(Clone, Debug, PartialEq)]
pub enum AppFocus {
    Input,
    History,
}

#[derive(Clone, Debug)]
pub enum StreamingState {
    Idle,
    Streaming { partial_content: String },
    Error(String),
}

#[derive(Clone, Debug)]
pub enum AppEvent {
    Key(KeyEvent),
    Stream {
        generation: u64,
        event: StreamEvent,
    },
}

pub struct App {
    pub conversation: Conversation,
    pub tool_registry: Arc<ToolRegistry>,
    pub textarea: TextArea<'static>,
    pub focus: AppFocus,
    pub streaming: StreamingState,
    pub stream_generation: u64,
    pub scroll_offset: u16,
    pub selected_message: Option<usize>,
    pub expanded_tool_blocks: HashSet<Uuid>,
    pub should_quit: bool,
}

impl App {
    pub fn new(tool_registry: Arc<ToolRegistry>) -> Self {
        let mut textarea = TextArea::default();
        textarea.set_placeholder_text("Type a message...");

        Self {
            conversation: Conversation::new(),
            tool_registry,
            textarea,
            focus: AppFocus::Input,
            streaming: StreamingState::Idle,
            stream_generation: 0,
            scroll_offset: 0,
            selected_message: None,
            expanded_tool_blocks: HashSet::new(),
            should_quit: false,
        }
    }

    pub fn handle_key_event(
        &mut self,
        key: KeyEvent,
        tx: &mpsc::Sender<AppEvent>,
        client: &LlmClient,
    ) {
        match self.focus {
            AppFocus::Input => self.handle_input_key(key, tx, client),
            AppFocus::History => self.handle_history_key(key),
        }
    }

    fn handle_input_key(
        &mut self,
        key: KeyEvent,
        tx: &mpsc::Sender<AppEvent>,
        client: &LlmClient,
    ) {
        match (key.code, key.modifiers) {
            (KeyCode::Char('c'), KeyModifiers::CONTROL) => {
                self.should_quit = true;
            }
            (KeyCode::Enter, KeyModifiers::SHIFT) => {
                // Insert newline
                self.textarea.input(Input {
                    key: Key::Enter,
                    ctrl: false,
                    alt: false,
                    shift: false,
                });
            }
            (KeyCode::Enter, KeyModifiers::NONE) => {
                self.send_message(tx, client);
            }
            (KeyCode::Tab, _) => {
                self.focus = AppFocus::History;
                if !self.conversation.is_empty() && self.selected_message.is_none() {
                    self.selected_message = Some(self.conversation.len().saturating_sub(1));
                }
            }
            (KeyCode::Esc, _) => {
                if matches!(self.streaming, StreamingState::Error(_)) {
                    self.streaming = StreamingState::Idle;
                }
            }
            _ => {
                let input = key_event_to_input(key);
                self.textarea.input(input);
            }
        }
    }

    fn handle_history_key(&mut self, key: KeyEvent) {
        match key.code {
            KeyCode::Up => {
                if let Some(idx) = self.selected_message {
                    if idx > 0 {
                        self.selected_message = Some(idx - 1);
                    }
                }
            }
            KeyCode::Down => {
                if let Some(idx) = self.selected_message {
                    if idx + 1 < self.conversation.len() {
                        self.selected_message = Some(idx + 1);
                    }
                }
            }
            KeyCode::Enter => {
                if let Some(idx) = self.selected_message {
                    if let Some(msg) = self.conversation.messages.get(idx) {
                        let id = msg.metadata.id;
                        if msg.tool_calls.is_some() || msg.tool_result.is_some() {
                            if self.expanded_tool_blocks.contains(&id) {
                                self.expanded_tool_blocks.remove(&id);
                            } else {
                                self.expanded_tool_blocks.insert(id);
                            }
                        }
                    }
                }
            }
            KeyCode::Tab => {
                self.focus = AppFocus::Input;
            }
            KeyCode::Esc => {
                self.focus = AppFocus::Input;
                self.selected_message = None;
            }
            KeyCode::Char('c') if key.modifiers.contains(KeyModifiers::CONTROL) => {
                self.should_quit = true;
            }
            _ => {}
        }
    }

    fn send_message(&mut self, tx: &mpsc::Sender<AppEvent>, client: &LlmClient) {
        if !matches!(self.streaming, StreamingState::Idle) {
            return;
        }

        let text: String = self.textarea.lines().join("\n");
        let text = text.trim().to_string();
        if text.is_empty() {
            return;
        }

        self.conversation.push(ChatMessage::user(&text));

        // Clear textarea
        self.textarea = TextArea::default();
        self.textarea.set_placeholder_text("Type a message...");

        self.stream_generation += 1;
        self.streaming = StreamingState::Streaming {
            partial_content: String::new(),
        };

        self.spawn_llm_stream(tx, client);
    }

    fn spawn_llm_stream(&self, tx: &mpsc::Sender<AppEvent>, client: &LlmClient) {
        let messages = self.conversation.to_openai_messages();
        let tools = if self.tool_registry.is_empty() {
            None
        } else {
            Some(self.tool_registry.to_openai_tools())
        };
        let generation = self.stream_generation;
        let tx = tx.clone();
        let client = client.clone();

        tokio::spawn(async move {
            let (stream_tx, mut stream_rx) = mpsc::channel::<StreamEvent>(100);

            tokio::spawn(async move {
                client.chat_stream(messages, tools, stream_tx).await;
            });

            while let Some(event) = stream_rx.recv().await {
                let _ = tx
                    .send(AppEvent::Stream {
                        generation,
                        event,
                    })
                    .await;
            }
        });
    }

    pub fn handle_stream_event(
        &mut self,
        generation: u64,
        event: StreamEvent,
        tx: &mpsc::Sender<AppEvent>,
        client: &LlmClient,
    ) {
        if generation != self.stream_generation {
            return;
        }

        match event {
            StreamEvent::Token(s) => {
                if let StreamingState::Streaming { partial_content } = &mut self.streaming {
                    partial_content.push_str(&s);
                }
            }
            StreamEvent::ToolCallStart { .. } | StreamEvent::ToolCallArgDelta { .. } => {
                // These are informational; the accumulator in llm_client handles assembly
            }
            StreamEvent::ToolCallComplete(call) => {
                self.handle_tool_call_complete(call, tx, client);
            }
            StreamEvent::ToolResultReady {
                tool_call_id,
                content,
            } => {
                self.handle_tool_result(tool_call_id, content, tx, client);
            }
            StreamEvent::Done { .. } => {
                self.finalize_streaming_content();
                self.streaming = StreamingState::Idle;
            }
            StreamEvent::Error(e) => {
                self.finalize_streaming_content();
                self.streaming = StreamingState::Error(e);
            }
        }
    }

    fn handle_tool_call_complete(
        &mut self,
        call: ToolCall,
        tx: &mpsc::Sender<AppEvent>,
        _client: &LlmClient,
    ) {
        // Finalize any partial streaming content
        self.finalize_streaming_content();

        // Push assistant message with tool calls
        let calls = vec![call.clone()];
        self.conversation
            .push(ChatMessage::assistant_tool_calls(calls));

        // Spawn tool execution
        let registry = self.tool_registry.clone();
        let generation = self.stream_generation;
        let tx = tx.clone();

        tokio::spawn(async move {
            let event = match registry.execute(&call).await {
                Ok(content) => StreamEvent::ToolResultReady {
                    tool_call_id: call.id,
                    content,
                },
                Err(e) => StreamEvent::Error(format!("Tool error: {}", e)),
            };
            let _ = tx
                .send(AppEvent::Stream {
                    generation,
                    event,
                })
                .await;
        });
    }

    fn handle_tool_result(
        &mut self,
        tool_call_id: String,
        content: String,
        tx: &mpsc::Sender<AppEvent>,
        client: &LlmClient,
    ) {
        self.conversation
            .push(ChatMessage::tool_result(&tool_call_id, &content));

        self.stream_generation += 1;
        self.streaming = StreamingState::Streaming {
            partial_content: String::new(),
        };

        self.spawn_llm_stream(tx, client);
    }

    fn finalize_streaming_content(&mut self) {
        if let StreamingState::Streaming { partial_content } = &self.streaming {
            if !partial_content.is_empty() {
                let content = partial_content.clone();
                self.conversation.push(ChatMessage::assistant(&content));
            }
        }
    }
}

fn key_event_to_input(key: KeyEvent) -> Input {
    let ctrl = key.modifiers.contains(KeyModifiers::CONTROL);
    let alt = key.modifiers.contains(KeyModifiers::ALT);
    let shift = key.modifiers.contains(KeyModifiers::SHIFT);

    let k = match key.code {
        KeyCode::Char(c) => Key::Char(c),
        KeyCode::Backspace => Key::Backspace,
        KeyCode::Enter => Key::Enter,
        KeyCode::Left => Key::Left,
        KeyCode::Right => Key::Right,
        KeyCode::Up => Key::Up,
        KeyCode::Down => Key::Down,
        KeyCode::Tab => Key::Tab,
        KeyCode::Delete => Key::Delete,
        KeyCode::Home => Key::Home,
        KeyCode::End => Key::End,
        KeyCode::PageUp => Key::PageUp,
        KeyCode::PageDown => Key::PageDown,
        KeyCode::Esc => Key::Esc,
        _ => Key::Null,
    };

    Input {
        key: k,
        ctrl,
        alt,
        shift,
    }
}
