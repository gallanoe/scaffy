use anyhow::Result;
use async_openai::types::chat::{ChatCompletionTool, ChatCompletionTools, FunctionObject};
use async_trait::async_trait;
use std::collections::HashMap;

use crate::conversation::ToolCall;

#[async_trait]
pub trait ToolHandler: Send + Sync {
    fn name(&self) -> &str;
    fn description(&self) -> &str;
    fn parameters_schema(&self) -> serde_json::Value;
    async fn execute(&self, args: serde_json::Value) -> Result<String>;
}

pub struct ToolRegistry {
    handlers: HashMap<String, Box<dyn ToolHandler>>,
}

impl ToolRegistry {
    pub fn new() -> Self {
        Self {
            handlers: HashMap::new(),
        }
    }

    pub fn register(&mut self, handler: impl ToolHandler + 'static) {
        self.handlers
            .insert(handler.name().to_string(), Box::new(handler));
    }

    pub fn get(&self, name: &str) -> Option<&dyn ToolHandler> {
        self.handlers.get(name).map(|h| h.as_ref())
    }

    pub fn to_openai_tools(&self) -> Vec<ChatCompletionTools> {
        self.handlers
            .values()
            .map(|handler| {
                ChatCompletionTools::Function(ChatCompletionTool {
                    function: FunctionObject {
                        name: handler.name().to_string(),
                        description: Some(handler.description().to_string()),
                        parameters: Some(handler.parameters_schema()),
                        strict: None,
                    },
                })
            })
            .collect()
    }

    pub async fn execute(&self, tool_call: &ToolCall) -> Result<String> {
        let handler = self
            .handlers
            .get(&tool_call.name)
            .ok_or_else(|| anyhow::anyhow!("Unknown tool: {}", tool_call.name))?;
        handler.execute(tool_call.arguments.clone()).await
    }

    pub fn is_empty(&self) -> bool {
        self.handlers.is_empty()
    }
}

pub struct EchoTool;

#[async_trait]
impl ToolHandler for EchoTool {
    fn name(&self) -> &str {
        "echo"
    }

    fn description(&self) -> &str {
        "Echoes back the provided arguments as a JSON string"
    }

    fn parameters_schema(&self) -> serde_json::Value {
        serde_json::json!({
            "type": "object",
            "properties": {
                "message": {
                    "type": "string",
                    "description": "The message to echo back"
                }
            },
            "required": ["message"]
        })
    }

    async fn execute(&self, args: serde_json::Value) -> Result<String> {
        Ok(serde_json::to_string_pretty(&args)?)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn test_echo_tool() {
        let tool = EchoTool;
        let args = serde_json::json!({"message": "hello"});
        let result = tool.execute(args.clone()).await.unwrap();
        let parsed: serde_json::Value = serde_json::from_str(&result).unwrap();
        assert_eq!(parsed, args);
    }

    #[tokio::test]
    async fn test_registry() {
        let mut registry = ToolRegistry::new();
        registry.register(EchoTool);

        assert!(registry.get("echo").is_some());
        assert!(registry.get("nonexistent").is_none());

        let call = ToolCall {
            id: "1".into(),
            name: "echo".into(),
            arguments: serde_json::json!({"message": "test"}),
        };
        let result = registry.execute(&call).await.unwrap();
        assert!(result.contains("test"));

        let tools = registry.to_openai_tools();
        assert_eq!(tools.len(), 1);
    }
}
