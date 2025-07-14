# Ch

⚠️ **Warning:** This project is currently in beta and is still being actively developed. It is in a rough state, and features may change or be unstable. Use at your own risk. For a more established version, you can check out the original [Cha](https://github.com/MehmetMHY/cha/).

## About

**Ch** is a GoLang implementation of the original Python-based [Cha](https://github.com/MehmetMHY/cha/). While not a 1-to-1 feature port, it contains over 79% of the core features of Cha and over 57% of the overall features, with significantly improved performance—delivering **2.55x faster** execution compared to the original Python version.

## Features

- **Multi-platform support**: OpenAI, Groq, DeepSeek, Anthropic, XAI
- **Interactive & Direct modes**: Chat interactively or run single queries
- **Web search integration**: SearXNG with IEEE citation format
- **File/directory loading**: Load text files into chat context with multi-select
- **Chat history export**: Export conversations to files
- **Text editor input mode**: Use your preferred editor for complex prompts
- **Model switching**: Easily switch between different AI models
- **Chat backtracking**: Revert to any point in the conversation history
- **Professional architecture**: Clean, modular Go codebase
- **Code Dump**: Get all the content/files in a directory and save it to a single string/text-file which can be used as context for the model(s)

## Requirements

- Go 1.21 or higher
- [fzf](https://github.com/junegunn/fzf) for interactive selections - `brew install fzf`
- API keys for your chosen platforms (OpenAI, Groq, etc.)
- (Optional) [Cha](https://github.com/MehmetMHY/cha/) for more advanced file content extraction (pdfs, images, videos, audios, etc).

## Installation

### Option 1: Build from source

```bash
# clone the repository
git clone https://github.com/MehmetMHY/ch.git
cd ch

# install dependencies
go mod download

# build the project
make build

# or use the build script
./build.sh

# or build manually
go build -o bin/ch cmd/ch/main.go
```

### Option 2: Install globally

```bash
# install to $GOPATH/bin
make install
```

## Configuration

### API Keys

Set up API keys for your chosen platforms:

```bash
export OPENAI_API_KEY="your-openai-key"
export GROQ_API_KEY="your-groq-key"
export DEEP_SEEK_API_KEY="your-deepseek-key"
export ANTHROPIC_API_KEY="your-anthropic-key"
export XAI_API_KEY="your-xai-key"
```

### Default Platform and Model

You can set default platform and model using environment variables:

```bash
export CH_DEFAULT_PLATFORM="groq"      # Default: "openai"
export CH_DEFAULT_MODEL="gemma2-9b-it" # Default: "gpt-4o-mini"
```

If not set, Ch will use OpenAI with gpt-4o-mini as defaults.

## Usage

### Basic Usage

```bash
# interactive mode
./bin/ch

# direct query
./bin/ch "What is artificial intelligence?"

# with specific platform
./bin/ch -p groq "Explain quantum computing"

# with specific model
./bin/ch -m gpt-4o "Write a Python function"

# combined
./bin/ch -p groq -m llama3 "What is the meaning of life?"
```

### Command Line Options

```bash
./bin/ch -h                                 # Show help
./bin/ch -d [directory]                     # Generate codedump file (current dir if no path)
./bin/ch -p [platform]                      # Switch platform
./bin/ch -m [model]                         # Specify model
./bin/ch -p [platform] -m [model] [query]   # Full command
```

### Interactive Commands

When running in interactive mode, you can use these commands:

- `!q` - Exit Interface
- `!h` - Help page with interactive selection
- `!c` - Clear chat history
- `!m` - Switch models (with fuzzy finder)
- `!p` - Switch platforms (with fuzzy finder)
- `!t` - Text editor input mode (opens your editor)
- `!b` - Backtrack to a previous message in the chat history
- `!l` - Load files/dirs into chat context
- `!d` - Generate codedump of all text files
- `!e [all]` - Save the last response or all history to a file
- `!s [query]` - Web search with AI analysis
- `\` - Enter multi-line input mode (exit by typing `\` on a new line)

#### Interactive Help Menu

The `!h` command opens an interactive help menu where you can:

- Select commands that start with `!` (without brackets) to execute them directly
- View commands with `[parameters]` by selecting them (displays info only)
- Commands without `!` prefix are informational only

## Web Search Setup

For web search functionality, set up SearXNG:

```bash
cd sxng
python3 run.py
```

Then use `!s <query>` in chat for web-enhanced responses with IEEE citations.

## Development

### Available Scripts

- `./build.sh`: Compiles the project and places the binary in the `bin/` directory.
- `./update.sh`: Updates all Go modules to their latest versions.

### Project Structure

```bash
ch/
├── cmd/ch/       # main application entry point
├── internal/     # internal packages
│   ├── config/   # configuration management
│   ├── chat/     # chat operations and history
│   ├── platform/ # AI platform integrations
│   ├── search/   # SearXNG web search
│   └── ui/       # terminal UI components
├── pkg/types/    # shared types and interfaces
├── sxng/         # SearXNG integration
└── Makefile      # build automation
```

### Available Make Commands

```bash
make build        # build the binary
make install      # install to $GOPATH/bin
make clean        # clean build artifacts
make test         # run tests
make lint         # run linter
make fmt          # format code
make vet          # run go vet
make deps         # download dependencies
make dev          # build and run in development mode
make build-all    # build for multiple platforms
make release      # create release tarballs
make help         # show all available commands
```

### Running Tests

```bash
make test   # run all tests
make lint   # run linter (requires golangci-lint)
make vet    # run go vet
```

## Examples

### Interactive Session

```bash
$ ./bin/ch
Chatting on OPENAI with gpt-4o-mini
!q - Exit
!m - Switch models
!p - Switch platforms
!t - Text editor input
!c - Clear history
!b - Backtrack
!h - Help
!l - Load files/dirs
!d - Generate codedump
!e [all] - Export chat
!s [query] - Web search

User: !p groq
# fuzzy finder opens with available platforms

User: !s latest developments in AI
# performs web search and provides AI analysis with citations

User: !d
# opens fzf selector to choose files/directories to exclude
# generates comprehensive codedump of project files

User: !t
# opens your preferred editor for complex input
```

### Direct Queries

```bash
# simple query
./bin/ch "Explain machine learning"

# with platform selection
./bin/ch -p anthropic "What are the ethical implications of AI?"

# with specific model
./bin/ch -p groq -m llama3 "Write a Go function to reverse a string"
```
