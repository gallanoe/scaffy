use anyhow::Result;
use async_openai::types::chat::{
    ChatCompletionMessageToolCall, ChatCompletionMessageToolCalls,
    ChatCompletionRequestAssistantMessage, ChatCompletionRequestAssistantMessageContent,
    ChatCompletionRequestMessage, ChatCompletionRequestSystemMessage,
    ChatCompletionRequestSystemMessageContent, ChatCompletionRequestToolMessage,
    ChatCompletionRequestToolMessageContent, ChatCompletionRequestUserMessage,
    ChatCompletionRequestUserMessageContent, CreateChatCompletionResponse, FunctionCall,
};
use serde::{Deserialize, Serialize};
use std::time::SystemTime;
use uuid::Uuid;

#[derive(Clone, Debug, Serialize, Deserialize)]
pub enum Role {
    System,
    User,
    Assistant,
    Tool,
}

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct ToolCall {
    pub id: String,
    pub name: String,
    pub arguments: serde_json::Value,
}

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct ToolResult {
    pub tool_call_id: String,
    pub content: String,
}

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct MessageMetadata {
    pub id: Uuid,
    pub timestamp: SystemTime,
    pub model: Option<String>,
    pub token_count: Option<u32>,
}

impl MessageMetadata {
    fn new() -> Self {
        Self {
            id: Uuid::new_v4(),
            timestamp: SystemTime::now(),
            model: None,
            token_count: None,
        }
    }
}

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct ChatMessage {
    pub role: Role,
    pub content: Option<String>,
    pub tool_calls: Option<Vec<ToolCall>>,
    pub tool_result: Option<ToolResult>,
    pub metadata: MessageMetadata,
}

impl ChatMessage {
    pub fn system(content: &str) -> Self {
        Self {
            role: Role::System,
            content: Some(content.to_string()),
            tool_calls: None,
            tool_result: None,
            metadata: MessageMetadata::new(),
        }
    }

    pub fn user(content: &str) -> Self {
        Self {
            role: Role::User,
            content: Some(content.to_string()),
            tool_calls: None,
            tool_result: None,
            metadata: MessageMetadata::new(),
        }
    }

    pub fn assistant(content: &str) -> Self {
        Self {
            role: Role::Assistant,
            content: Some(content.to_string()),
            tool_calls: None,
            tool_result: None,
            metadata: MessageMetadata::new(),
        }
    }

    pub fn assistant_tool_calls(calls: Vec<ToolCall>) -> Self {
        Self {
            role: Role::Assistant,
            content: None,
            tool_calls: Some(calls),
            tool_result: None,
            metadata: MessageMetadata::new(),
        }
    }

    pub fn tool_result(tool_call_id: &str, content: &str) -> Self {
        Self {
            role: Role::Tool,
            content: None,
            tool_calls: None,
            tool_result: Some(ToolResult {
                tool_call_id: tool_call_id.to_string(),
                content: content.to_string(),
            }),
            metadata: MessageMetadata::new(),
        }
    }
}

#[derive(Clone, Debug, Default)]
pub struct Conversation {
    pub messages: Vec<ChatMessage>,
}

impl Conversation {
    pub fn new() -> Self {
        Self {
            messages: Vec::new(),
        }
    }

    pub fn push(&mut self, message: ChatMessage) {
        self.messages.push(message);
    }

    pub fn len(&self) -> usize {
        self.messages.len()
    }

    pub fn is_empty(&self) -> bool {
        self.messages.is_empty()
    }

    pub fn last_assistant_message(&self) -> Option<&ChatMessage> {
        self.messages
            .iter()
            .rev()
            .find(|m| matches!(m.role, Role::Assistant))
    }

    pub fn clear(&mut self) {
        self.messages.clear();
    }

    pub fn to_openai_messages(&self) -> Vec<ChatCompletionRequestMessage> {
        self.messages
            .iter()
            .map(|msg| match &msg.role {
                Role::System => ChatCompletionRequestMessage::System(
                    ChatCompletionRequestSystemMessage {
                        content: ChatCompletionRequestSystemMessageContent::Text(
                            msg.content.clone().unwrap_or_default(),
                        ),
                        name: None,
                    },
                ),
                Role::User => {
                    ChatCompletionRequestMessage::User(ChatCompletionRequestUserMessage {
                        content: ChatCompletionRequestUserMessageContent::Text(
                            msg.content.clone().unwrap_or_default(),
                        ),
                        name: None,
                    })
                }
                Role::Assistant => {
                    let tool_calls = msg.tool_calls.as_ref().map(|calls| {
                        calls
                            .iter()
                            .map(|call| {
                                ChatCompletionMessageToolCalls::Function(
                                    ChatCompletionMessageToolCall {
                                        id: call.id.clone(),
                                        function: FunctionCall {
                                            name: call.name.clone(),
                                            arguments: call.arguments.to_string(),
                                        },
                                    },
                                )
                            })
                            .collect()
                    });
                    #[allow(deprecated)]
                    ChatCompletionRequestMessage::Assistant(
                        ChatCompletionRequestAssistantMessage {
                            content: msg.content.as_ref().map(|c| {
                                ChatCompletionRequestAssistantMessageContent::Text(c.clone())
                            }),
                            tool_calls,
                            name: None,
                            refusal: None,
                            audio: None,
                            function_call: None,
                        },
                    )
                }
                Role::Tool => {
                    let result = msg.tool_result.as_ref().expect("Tool message must have result");
                    ChatCompletionRequestMessage::Tool(ChatCompletionRequestToolMessage {
                        content: ChatCompletionRequestToolMessageContent::Text(
                            result.content.clone(),
                        ),
                        tool_call_id: result.tool_call_id.clone(),
                    })
                }
            })
            .collect()
    }

    pub fn push_from_response(&mut self, response: &CreateChatCompletionResponse) -> Result<()> {
        let choice = response
            .choices
            .first()
            .ok_or_else(|| anyhow::anyhow!("No choices in response"))?;
        let resp_msg = &choice.message;

        if let Some(tool_calls) = &resp_msg.tool_calls {
            let calls: Vec<ToolCall> = tool_calls
                .iter()
                .filter_map(|tc| match tc {
                    ChatCompletionMessageToolCalls::Function(call) => {
                        let args = serde_json::from_str(&call.function.arguments)
                            .unwrap_or(serde_json::Value::String(
                                call.function.arguments.clone(),
                            ));
                        Some(ToolCall {
                            id: call.id.clone(),
                            name: call.function.name.clone(),
                            arguments: args,
                        })
                    }
                    _ => None,
                })
                .collect();

            if !calls.is_empty() {
                let mut msg = ChatMessage::assistant_tool_calls(calls);
                msg.metadata.model = Some(response.model.clone());
                if let Some(usage) = &response.usage {
                    msg.metadata.token_count = Some(usage.completion_tokens);
                }
                self.push(msg);
                return Ok(());
            }
        }

        if let Some(content) = &resp_msg.content {
            let mut msg = ChatMessage::assistant(content);
            msg.metadata.model = Some(response.model.clone());
            if let Some(usage) = &response.usage {
                msg.metadata.token_count = Some(usage.completion_tokens);
            }
            self.push(msg);
        }

        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_constructors() {
        let msg = ChatMessage::system("test");
        assert!(matches!(msg.role, Role::System));
        assert_eq!(msg.content.as_deref(), Some("test"));

        let msg = ChatMessage::user("hello");
        assert!(matches!(msg.role, Role::User));

        let msg = ChatMessage::assistant("response");
        assert!(matches!(msg.role, Role::Assistant));
        assert!(msg.tool_calls.is_none());

        let calls = vec![ToolCall {
            id: "1".into(),
            name: "echo".into(),
            arguments: serde_json::json!({"text": "hi"}),
        }];
        let msg = ChatMessage::assistant_tool_calls(calls);
        assert!(msg.tool_calls.is_some());
        assert!(msg.content.is_none());

        let msg = ChatMessage::tool_result("1", "result");
        assert!(matches!(msg.role, Role::Tool));
        assert_eq!(msg.tool_result.as_ref().unwrap().tool_call_id, "1");
    }

    #[test]
    fn test_conversation() {
        let mut conv = Conversation::new();
        assert!(conv.is_empty());

        conv.push(ChatMessage::user("hi"));
        conv.push(ChatMessage::assistant("hello"));
        assert_eq!(conv.len(), 2);
        assert_eq!(
            conv.last_assistant_message().unwrap().content.as_deref(),
            Some("hello")
        );
    }

    #[test]
    fn test_to_openai_messages() {
        let mut conv = Conversation::new();
        conv.push(ChatMessage::system("sys"));
        conv.push(ChatMessage::user("usr"));
        conv.push(ChatMessage::assistant("asst"));
        conv.push(ChatMessage::tool_result("tc1", "result"));

        let msgs = conv.to_openai_messages();
        assert_eq!(msgs.len(), 4);
        assert!(matches!(msgs[0], ChatCompletionRequestMessage::System(_)));
        assert!(matches!(msgs[1], ChatCompletionRequestMessage::User(_)));
        assert!(matches!(
            msgs[2],
            ChatCompletionRequestMessage::Assistant(_)
        ));
        assert!(matches!(msgs[3], ChatCompletionRequestMessage::Tool(_)));
    }
}
