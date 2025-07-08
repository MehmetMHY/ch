# Simple Go Chat Client

A simple Go implementation of a chat client with OpenAI API integration, based on the original Python Cha project.

## Features

- **Chat with OpenAI models** - Interactive chat interface with streaming responses
- **Model switching** - Switch between different OpenAI models during conversation
- **Terminal input mode (!t)** - Multi-line input mode for longer messages
- **Command interface** - Special commands for various functions
- **Chat history** - Maintains conversation context
- **Fuzzy model selection** - Uses fzf for model selection

## Prerequisites

- Go 1.21 or later
- OpenAI API key
- `fzf` command-line tool for model selection

## Installation

1. Clone or download the files
2. Set your OpenAI API key:
   ```bash
   export OPENAI_API_KEY="your-api-key-here"
   ```
3. Install dependencies:
   ```bash
   go mod tidy
   ```
4. Run the application:
   ```bash
   go run main.go
   ```

## Usage

### Basic Commands

- `!q` - Exit the application
- `!h` - Show help and available commands
- `!c` - Clear chat history
- `!sm` - Interactive model selection (uses fzf)
- `!sm <model-name>` - Switch to specific model directly
- `!t` - Terminal input mode (multi-line input, finish with Ctrl+D)

### Example Usage

```bash
$ go run main.go
=== Simple Go Chat Client ===
Chatting with OpenAI Model: gpt-4
Commands:
- '!q' to exit
- '!sm' to switch models
- '!t' for terminal input mode
- '!c' to clear chat history
- '!h' for help

User: Hello, how are you?
Hello! I'm doing well, thank you for asking. How can I help you today?

User: !sm
Fetching available models...
# fzf interface appears to select model

User: !t
Terminal input mode - Enter your message (Ctrl+D to finish):
This is a longer message
that spans multiple lines
and can contain complex formatting
^D
> This is a longer message
> that spans multiple lines  
> and can contain complex formatting
[AI response appears here]
```

## Configuration

The application uses environment variables and default configuration:

- `OPENAI_API_KEY` - Required: Your OpenAI API key
- Default model: `gpt-4`
- System prompt: "You are a helpful assistant powered by a simple Go chat client. Be concise, clear, and accurate."

## Dependencies

- `github.com/sashabaranov/go-openai` - OpenAI API client for Go

## Notes

- Requires `fzf` for model selection functionality
- Responses are streamed in real-time
- Chat history is maintained during the session
- Terminal input mode allows for multi-line messages
- Color-coded output for better readability