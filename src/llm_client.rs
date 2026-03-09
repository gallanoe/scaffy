use anyhow::Result;
use async_openai::{
    config::OpenAIConfig,
    types::chat::{
        ChatCompletionMessageToolCallChunk, ChatCompletionRequestMessage, ChatCompletionTools,
        CreateChatCompletionRequestArgs, FinishReason,
    },
    Client,
};
use futures::StreamExt;
use std::collections::HashMap;
use tokio::sync::mpsc;

use crate::conversation::ToolCall;

#[derive(Clone, Debug)]
pub enum StreamEvent {
    Token(String),
    ToolCallStart {
        index: usize,
        id: String,
        name: String,
    },
    ToolCallArgDelta {
        index: usize,
        arguments: String,
    },
    ToolCallComplete(ToolCall),
    ToolResultReady {
        tool_call_id: String,
        content: String,
    },
    Done {
        stop_reason: Option<String>,
    },
    Error(String),
}

struct PartialToolCall {
    id: String,
    name: String,
    arguments: String,
}

struct ToolCallAccumulator {
    calls: HashMap<u32, PartialToolCall>,
}

impl ToolCallAccumulator {
    fn new() -> Self {
        Self {
            calls: HashMap::new(),
        }
    }

    fn process_delta(&mut self, chunk: &ChatCompletionMessageToolCallChunk) -> Option<StreamEvent> {
        let index = chunk.index as usize;

        if let Some(id) = &chunk.id {
            let name = chunk
                .function
                .as_ref()
                .and_then(|f| f.name.as_ref())
                .cloned()
                .unwrap_or_default();

            self.calls.insert(
                chunk.index,
                PartialToolCall {
                    id: id.clone(),
                    name: name.clone(),
                    arguments: String::new(),
                },
            );

            return Some(StreamEvent::ToolCallStart {
                index,
                id: id.clone(),
                name,
            });
        }

        if let Some(func) = &chunk.function {
            if let Some(args) = &func.arguments {
                if let Some(partial) = self.calls.get_mut(&chunk.index) {
                    partial.arguments.push_str(args);
                    return Some(StreamEvent::ToolCallArgDelta {
                        index,
                        arguments: args.clone(),
                    });
                }
            }
        }

        None
    }

    fn finalize(self) -> Vec<ToolCall> {
        self.calls
            .into_values()
            .map(|p| {
                let arguments = serde_json::from_str(&p.arguments)
                    .unwrap_or(serde_json::Value::String(p.arguments));
                ToolCall {
                    id: p.id,
                    name: p.name,
                    arguments,
                }
            })
            .collect()
    }
}

#[derive(Clone, Debug)]
pub struct LlmClientConfig {
    pub base_url: String,
    pub api_key: String,
    pub model: String,
    pub max_tokens: u32,
    pub temperature: f32,
    pub system_prompt: Option<String>,
}

#[derive(Clone)]
pub struct LlmClient {
    client: Client<OpenAIConfig>,
    pub config: LlmClientConfig,
}

impl LlmClient {
    pub fn new(config: LlmClientConfig) -> Result<Self> {
        let openai_config = OpenAIConfig::new()
            .with_api_key(&config.api_key)
            .with_api_base(&config.base_url);

        let client = Client::with_config(openai_config);

        Ok(Self { client, config })
    }

    pub async fn chat_stream(
        &self,
        messages: Vec<ChatCompletionRequestMessage>,
        tools: Option<Vec<ChatCompletionTools>>,
        tx: mpsc::Sender<StreamEvent>,
    ) {
        let result = self.do_chat_stream(messages, tools, &tx).await;
        if let Err(e) = result {
            let _ = tx.send(StreamEvent::Error(e.to_string())).await;
        }
    }

    async fn do_chat_stream(
        &self,
        messages: Vec<ChatCompletionRequestMessage>,
        tools: Option<Vec<ChatCompletionTools>>,
        tx: &mpsc::Sender<StreamEvent>,
    ) -> Result<()> {
        let mut args = CreateChatCompletionRequestArgs::default();
        args.model(&self.config.model)
            .messages(messages)
            .max_completion_tokens(self.config.max_tokens)
            .temperature(self.config.temperature);

        if let Some(tools) = tools {
            if !tools.is_empty() {
                args.tools(tools);
            }
        }

        let request = args.build()?;
        let mut stream = self.client.chat().create_stream(request).await?;
        let mut accumulator = ToolCallAccumulator::new();

        while let Some(result) = stream.next().await {
            match result {
                Ok(response) => {
                    if let Some(choice) = response.choices.first() {
                        // Handle content tokens
                        if let Some(content) = &choice.delta.content {
                            if !content.is_empty() {
                                let _ = tx.send(StreamEvent::Token(content.clone())).await;
                            }
                        }

                        // Handle tool call deltas
                        if let Some(tool_calls) = &choice.delta.tool_calls {
                            for chunk in tool_calls {
                                if let Some(event) = accumulator.process_delta(chunk) {
                                    let _ = tx.send(event).await;
                                }
                            }
                        }

                        // Handle finish reason
                        if let Some(reason) = &choice.finish_reason {
                            match reason {
                                FinishReason::ToolCalls => {
                                    let calls = accumulator.finalize();
                                    for call in calls {
                                        let _ = tx
                                            .send(StreamEvent::ToolCallComplete(call))
                                            .await;
                                    }
                                    let _ = tx
                                        .send(StreamEvent::Done {
                                            stop_reason: Some("tool_calls".to_string()),
                                        })
                                        .await;
                                    return Ok(());
                                }
                                FinishReason::Stop => {
                                    let _ = tx
                                        .send(StreamEvent::Done {
                                            stop_reason: Some("stop".to_string()),
                                        })
                                        .await;
                                    return Ok(());
                                }
                                other => {
                                    let _ = tx
                                        .send(StreamEvent::Done {
                                            stop_reason: Some(format!("{:?}", other)),
                                        })
                                        .await;
                                    return Ok(());
                                }
                            }
                        }
                    }
                }
                Err(e) => {
                    let _ = tx.send(StreamEvent::Error(e.to_string())).await;
                    return Ok(());
                }
            }
        }

        // Stream ended without explicit finish reason
        let _ = tx.send(StreamEvent::Done { stop_reason: None }).await;
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use async_openai::types::chat::FunctionCallStream;

    #[test]
    fn test_tool_call_accumulator() {
        let mut acc = ToolCallAccumulator::new();

        // First chunk: tool call start
        let chunk = ChatCompletionMessageToolCallChunk {
            index: 0,
            id: Some("call_123".to_string()),
            r#type: None,
            function: Some(FunctionCallStream {
                name: Some("echo".to_string()),
                arguments: None,
            }),
        };
        let event = acc.process_delta(&chunk);
        assert!(matches!(event, Some(StreamEvent::ToolCallStart { .. })));

        // Second chunk: argument delta
        let chunk = ChatCompletionMessageToolCallChunk {
            index: 0,
            id: None,
            r#type: None,
            function: Some(FunctionCallStream {
                name: None,
                arguments: Some(r#"{"message": "#.to_string()),
            }),
        };
        let event = acc.process_delta(&chunk);
        assert!(matches!(event, Some(StreamEvent::ToolCallArgDelta { .. })));

        // Third chunk: more arguments
        let chunk = ChatCompletionMessageToolCallChunk {
            index: 0,
            id: None,
            r#type: None,
            function: Some(FunctionCallStream {
                name: None,
                arguments: Some(r#""hello"}"#.to_string()),
            }),
        };
        acc.process_delta(&chunk);

        // Finalize
        let calls = acc.finalize();
        assert_eq!(calls.len(), 1);
        assert_eq!(calls[0].id, "call_123");
        assert_eq!(calls[0].name, "echo");
        assert_eq!(calls[0].arguments, serde_json::json!({"message": "hello"}));
    }
}
