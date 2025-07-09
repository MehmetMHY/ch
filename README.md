# Ch

A professional Go CLI chat client supporting multiple AI platforms with web search integration.

**Ch** is a GoLang implementation of the original Python-based [Cha](https://github.com/MehmetMHY/cha/). While not a 1-to-1 feature port, it contains 95%+ of the core features of Cha with significantly improved performance - delivering **6.84x faster** execution compared to the original Python version.

## Features

- **Multi-platform support**: OpenAI, Groq, DeepSeek, Anthropic, XAI
- **Interactive & Direct modes**: Chat interactively or run single queries
- **Web search integration**: SearXNG with IEEE citation format
- **File/directory loading**: Load text files into chat context with multi-select
- **Chat history export**: Export conversations to files
- **Terminal input mode**: Use your preferred editor for complex prompts
- **Model switching**: Easily switch between different AI models
- **Professional architecture**: Clean, modular Go codebase

## Requirements

- Go 1.21 or higher
- [fzf](https://github.com/junegunn/fzf) for interactive selections - `brew install fzf`
- API keys for your chosen platforms (OpenAI, Groq, etc.)

## Installation

### Option 1: Build from source

```bash
# Clone the repository
git clone https://github.com/MehmetMHY/ch.git
cd ch

# Install dependencies
go mod download

# Build the project
make build

# Or use the build script
./build.sh

# Or build manually
go build -o bin/ch cmd/ch/main.go
```

### Option 2: Install globally

```bash
# Install to $GOPATH/bin
make install

# Or install manually
go install github.com/MehmetMHY/ch/cmd/ch@latest
```

## Configuration

Set up API keys for your chosen platforms:

```bash
export OPENAI_API_KEY="your-openai-key"
export GROQ_API_KEY="your-groq-key"
export DEEP_SEEK_API_KEY="your-deepseek-key"
export ANTHROPIC_API_KEY="your-anthropic-key"
export XAI_API_KEY="your-xai-key"
```

## Usage

### Basic Usage

```bash
# Interactive mode
./bin/ch

# Direct query
./bin/ch "What is artificial intelligence?"

# With specific platform
./bin/ch -p groq "Explain quantum computing"

# With specific model
./bin/ch -m gpt-4o "Write a Python function"

# Combined
./bin/ch -p groq -m llama3 "What is the meaning of life?"
```

### Command Line Options

```bash
./bin/ch -h                              # Show help
./bin/ch -p [platform]                   # Switch platform
./bin/ch -m [model]                      # Specify model
./bin/ch -p [platform] -m [model] [query]  # Full command
```

### Interactive Commands

When running in interactive mode, you can use these commands:

- `!q` - Exit the application
- `!h` - Show help
- `!c` - Clear chat history
- `!sm` - Switch models (with fuzzy finder)
- `!p` - Switch platforms (with fuzzy finder)
- `!t` - Terminal input mode (opens your editor)
- `!e` - Export chat history to file
- `!w [query]` - Web search with AI analysis
- `!l` - Load files/directories into chat context

## Web Search Setup

For web search functionality, set up SearXNG:

```bash
cd assets/sxng
python3 run.py
```

Then use `!w <query>` in chat for web-enhanced responses with IEEE citations.

## Development

### Project Structure

```
ch/
├── cmd/ch/          # Main application entry point
├── internal/            # Internal packages
│   ├── config/         # Configuration management
│   ├── chat/           # Chat operations and history
│   ├── platform/       # AI platform integrations
│   ├── search/         # SearXNG web search
│   └── ui/             # Terminal UI components
├── pkg/types/          # Shared types and interfaces
├── assets/sxng/        # SearXNG integration
└── Makefile           # Build automation
```

### Available Make Commands

```bash
make build       # Build the binary
make install     # Install to $GOPATH/bin
make clean       # Clean build artifacts
make test        # Run tests
make lint        # Run linter
make fmt         # Format code
make vet         # Run go vet
make deps        # Download dependencies
make dev         # Build and run in development mode
make build-all   # Build for multiple platforms
make release     # Create release tarballs
make help        # Show all available commands
```

### Running Tests

```bash
make test        # Run all tests
make lint        # Run linter (requires golangci-lint)
make vet         # Run go vet
```

## Examples

### Interactive Session

```bash
$ ./bin/ch
Chatting with OPENAI Model: gpt-4o-mini
Commands:
  • !q - Exit
  • !sm - Switch models
  • !p - Switch platforms
  • !t - Terminal input
  • !c - Clear history
  • !e - Export chat
  • !h - Help
  • !w [query] - Web search
  • !l - Load files/directories

User: !p groq
# Fuzzy finder opens with available platforms

User: !w latest developments in AI
# Performs web search and provides AI analysis with citations

User: !t
# Opens your preferred editor for complex input
```

### Direct Queries

```bash
# Simple query
./bin/ch "Explain machine learning"

# With platform selection
./bin/ch -p anthropic "What are the ethical implications of AI?"

# With specific model
./bin/ch -p groq -m llama3 "Write a Go function to reverse a string"
```
