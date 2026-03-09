#[allow(dead_code)]
mod app;
#[allow(dead_code)]
mod conversation;
#[allow(dead_code)]
mod llm_client;
#[allow(dead_code)]
mod tools;
mod ui;

use std::io;
use std::sync::Arc;
use std::time::Duration;

use anyhow::Result;
use crossterm::{
    event::{self, Event},
    execute,
    terminal::{disable_raw_mode, enable_raw_mode, EnterAlternateScreen, LeaveAlternateScreen},
};
use ratatui::{backend::CrosstermBackend, Terminal};
use tokio::sync::mpsc;

use app::{App, AppEvent};
use llm_client::{LlmClient, LlmClientConfig};
use tools::{EchoTool, ToolRegistry};

#[tokio::main]
async fn main() -> Result<()> {
    // Load .env
    dotenvy::dotenv().ok();

    // Init tracing to file (not stdout — stdout is the TUI)
    let log_file = std::fs::File::create("scaffy.log")?;
    tracing_subscriber::fmt()
        .with_writer(log_file)
        .with_ansi(false)
        .init();

    // Build LLM client config from env
    let api_key = std::env::var("OPENROUTER_API_KEY")
        .expect("OPENROUTER_API_KEY must be set in .env or environment");
    let base_url = std::env::var("OPENROUTER_BASE_URL")
        .unwrap_or_else(|_| "https://openrouter.ai/api/v1".to_string());
    let model =
        std::env::var("SCAFFY_MODEL").unwrap_or_else(|_| "anthropic/claude-sonnet-4".to_string());
    let max_tokens: u32 = std::env::var("SCAFFY_MAX_TOKENS")
        .ok()
        .and_then(|v| v.parse().ok())
        .unwrap_or(4096);
    let temperature: f32 = std::env::var("SCAFFY_TEMPERATURE")
        .ok()
        .and_then(|v| v.parse().ok())
        .unwrap_or(0.7);
    let system_prompt = std::env::var("SCAFFY_SYSTEM_PROMPT").ok();

    let config = LlmClientConfig {
        base_url,
        api_key,
        model,
        max_tokens,
        temperature,
        system_prompt: system_prompt.clone(),
    };
    let client = LlmClient::new(config)?;

    // Build tool registry
    // Register EchoTool only when SCAFFY_ECHO_TOOL=1 (it's a test/demo tool)
    let mut registry = ToolRegistry::new();
    if std::env::var("SCAFFY_ECHO_TOOL").as_deref() == Ok("1") {
        registry.register(EchoTool);
    }
    let registry = Arc::new(registry);

    // Create app state
    let mut app = App::new(registry);

    // Add system prompt if configured
    if let Some(prompt) = system_prompt {
        app.conversation
            .push(conversation::ChatMessage::system(&prompt));
    }

    // Setup terminal
    enable_raw_mode()?;
    let mut stdout = io::stdout();
    execute!(stdout, EnterAlternateScreen)?;
    let backend = CrosstermBackend::new(stdout);
    let mut terminal = Terminal::new(backend)?;

    // Set panic hook to restore terminal
    let original_hook = std::panic::take_hook();
    std::panic::set_hook(Box::new(move |panic_info| {
        let _ = disable_raw_mode();
        let _ = execute!(io::stdout(), LeaveAlternateScreen);
        original_hook(panic_info);
    }));

    // Event channel
    let (tx, mut rx) = mpsc::channel::<AppEvent>(100);

    // Spawn terminal input polling task
    let input_tx = tx.clone();
    tokio::spawn(async move {
        loop {
            if event::poll(Duration::from_millis(50)).unwrap_or(false) {
                if let Ok(Event::Key(key)) = event::read() {
                    if input_tx.send(AppEvent::Key(key)).await.is_err() {
                        break;
                    }
                }
            }
        }
    });

    // Main event loop
    loop {
        terminal.draw(|f| ui::render(f, &app))?;

        if let Some(event) = rx.recv().await {
            match event {
                AppEvent::Key(key) => {
                    app.handle_key_event(key, &tx, &client);
                }
                AppEvent::Stream { generation, event } => {
                    app.handle_stream_event(generation, event, &tx, &client);
                }
            }
        }

        if app.should_quit {
            break;
        }
    }

    // Restore terminal
    disable_raw_mode()?;
    execute!(terminal.backend_mut(), LeaveAlternateScreen)?;
    terminal.show_cursor()?;

    Ok(())
}
