# Ch

<a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License: MIT"></a>
<a href="https://golang.org/"><img src="https://img.shields.io/badge/go-1.21+-blue.svg" alt="Go 1.21+"></a>

## Table of Contents

- [Overview](#overview)
- [Vision](#vision)
- [Quick Start](#quick-start)
- [Features](#features)
- [Installation](#installation)
- [Configuration](#configuration)
  - [API Keys](#api-keys)
  - [Default Settings](#default-settings)
  - [Local \& Open-Source Setup (Ollama)](#local--open-source-setup-ollama)
- [Usage](#usage)
  - [Basic Usage](#basic-usage)
  - [Interactive Commands](#interactive-commands)
  - [Advanced Features](#advanced-features)
- [Web Scraping](#web-scraping)
- [Platform Compatibility](#platform-compatibility)
- [Development](#development)
  - [Prerequisites](#prerequisites)
  - [Build from Source](#build-from-source)
  - [Build Options](#build-options)
- [Contributing](#contributing)
  - [Development Setup](#development-setup)
  - [Code Standards](#code-standards)
- [License](#license)

## Overview

**Ch** is a lightweight, GoLang-based CLI tool for AI interaction. As the successor to the now-deprecated [Cha](https://github.com/MehmetMHY/cha/) project, Ch delivers the same core functionality with over 10x faster startup and significantly improved performance. Ch prioritizes speed and efficiency, making it ideal for developers who need rapid AI interaction with minimal overhead and full user control.

## Vision

Ch provides direct terminal access to powerful AI models with minimal overhead, transparent operations, and explicit user control. It integrates seamlessly into developer environments, minimizing context switching and empowering users to leverage AI's full potential through explicit control and flexible, user-driven interactions without automated decisions or hidden costs.

## Quick Start

**Install:**

```bash
curl -fsSL https://raw.githubusercontent.com/MehmetMHY/ch/main/install.sh | bash
```

**Configure:**

```bash
export OPENAI_API_KEY="your-api-key-here"
```

**Start using:**

```bash
ch "What are the key features of Go programming language?"
```

## Features

- **High Performance**: Built for speed with minimal startup overhead
- **Multi-Platform Support**: OpenAI, Groq, DeepSeek, Anthropic, XAI, Together, Google Gemini, Mistral AI, and Ollama
- **Interactive & Direct Modes**: Chat interactively or run single queries
- **Unix Piping**: Pipe any command output or file content directly to Ch
- **Smart File Handling**: Load text files, PDFs, Word docs, spreadsheets (XLSX/CSV), and directories
- **Advanced Export**: Interactive chat export with fzf selection and editor integration
- **Code Block Export**: Extract and save markdown code blocks with proper file extensions
- **Chat History Viewer**: Interactive display of conversation history with filtering and search
- **Token Counting**: Estimate token usage for files with model-aware tokenization
- **Text Editor Integration**: Use your preferred editor for complex prompts
- **Dynamic Switching**: Change models and platforms mid-conversation
- **Chat Backtracking**: Revert to any point in conversation history
- **Code Dump**: Package entire directories for AI analysis
- **Shell Session Recording**: Record terminal sessions and provide them as context to the model
- **Web Scraping & Search**: Built-in URL scraping and web search capabilities
- **Clipboard Integration**: Copy AI responses to clipboard with cross-platform support
- **Colored Output**: Platform and model names displayed in distinct colors

## Installation

```bash
curl -fsSL https://raw.githubusercontent.com/MehmetMHY/ch/main/install.sh | bash
```

**Alternative methods:**

```bash
# using wget
wget -qO- https://raw.githubusercontent.com/MehmetMHY/ch/main/install.sh | bash

# manual clone and install
git clone https://github.com/MehmetMHY/ch.git
cd ch
./install.sh
```

**Uninstall:**

```bash
# run installer with the uninstall flag
curl -fsSL https://raw.githubusercontent.com/MehmetMHY/ch/main/install.sh | bash -s -- --uninstall

# or if you have the installer script locally
./install.sh --uninstall
```

The installer automatically:

- Checks for Go 1.21+ and dependencies (fzf, curl, lynx, yt-dlp, ddgr)
- Installs missing dependencies via system package managers (apt, brew, pkg, etc.)
- Builds and installs Ch to `~/.ch/bin/ch` with temporary files in `~/.ch/tmp/`
- Creates global symlink at `/usr/local/bin/ch` (or `$PREFIX/bin/ch` on Android/Termux)
- Configures PATH if needed

## Configuration

### API Keys

Set up API keys for your chosen platforms:

```bash
# required
export OPENAI_API_KEY="your-openai-key"

# optional additional platforms
export GROQ_API_KEY="your-groq-key"
export DEEP_SEEK_API_KEY="your-deepseek-key"
export ANTHROPIC_API_KEY="your-anthropic-key"
export XAI_API_KEY="your-xai-key"
export TOGETHER_API_KEY="your-together-key"
export GEMINI_API_KEY="your-gemini-key"
export MISTRAL_API_KEY="your-mistral-key"
```

### Default Settings

Customize default platform and model:

```bash
# default: openai
export CH_DEFAULT_PLATFORM="groq"

# default: gpt-4o-mini
export CH_DEFAULT_MODEL="llama3-8b-8192"
```

### Local & Open-Source Setup (Ollama)

Ch supports local models via Ollama, allowing you to run it without relying on third-party services. This provides a completely private, open-source, and offline-capable environment.

1.  **Install Ollama**: Follow the official instructions at [ollama.com](https://ollama.com).
2.  **Pull a model**: `ollama pull llama3`

3.  **Run Ch with Ollama**: `ch -p ollama "What is the capital of France?"`

Since Ollama runs locally, no API key is required.

## Usage

### Basic Usage

```bash
# interactive mode
ch

# direct query
ch "Explain quantum computing"

# platform-specific query
ch -p groq "Write a Go function to reverse a string"

# model-specific query
ch -m gpt-4o "Create a REST API in Python"

# export code blocks to files
ch -e "Write a Python script to sort a list"

# load and display file content
ch -l document.pdf
ch -l spreadsheet.xlsx

# scrape web content
ch -l https://example.com
ch -l https://youtube.com/watch?v=example

# count tokens in files
ch -t ./README.md
ch -m "gpt-4" -t ./main.go

# piping support
cat main.py | ch "What does this code do?"
echo "hello world" | ch "Translate to Spanish"
ls -la | ch "Summarize this directory"
```

### Interactive Commands

When in interactive mode (`ch`), use these commands:

- **`!q`** - Exit interface
- **`!h`** - Interactive help menu
- **`!m`** - Switch models (with fuzzy finder)
- **`!p`** - Switch platforms (with fuzzy finder)
- **`!c`** - Clear chat history
- **`!t`** - Text editor input mode
- **`!b`** - Backtrack to previous message
- **`!l`** - Load files from current directory
- **`!l <dir>`** - Load files from specified directory
- **`!d`** - Generate code dump
- **`!e`** - Export selected chat entries
- **`!s`** - Scrape content from URLs (supports multiple URLs and YouTube)
- **`!w`** - Search web using DuckDuckGo
- **`!y`** - Copy selected responses to clipboard
- **`!x`** - Record shell session or run command (`!x ls` streams output live)
- **`\`** - Multi-line input mode
- **`|`** - View and display chat history
- **`Ctrl+C`** - Clear current prompt input
- **`Ctrl+D`** - Exit interface

### Advanced Features

**Code Export (`-e` flag):**

- Automatically detects programming languages
- Saves with proper file extensions
- Supports 25+ languages and file types

**Interactive Export (`!e`):**

1. Select chat entries with fzf
2. Edit content in your preferred editor
3. Choose existing file or create new file with suggested names

**Chat History Viewer (`|`):**

1. Browse complete conversation history with exact search
2. Filter by user messages, bot responses, or loaded files
3. Display individual entries or complete formatted history
4. Shows platform/model changes and file loading events

**URL Scraping (`!s` and `-l` with URLs):**

- Supports regular web pages and YouTube videos
- Extracts clean text content using curl and lynx
- YouTube videos include metadata and subtitle extraction via yt-dlp
- Multiple URL support: `!s https://site1.com https://site2.com`
- Integrated with file loading: `ch -l https://example.com`

**Web Search (`!w`):**

- Built-in DuckDuckGo search integration via ddgr
- Usage: `!w "search query"`
- Results are automatically added to conversation context
- No need for external tools or APIs

**Clipboard Copy (`!y`):**

- Select one or more AI responses with fzf
- Edit content in your preferred editor before copying
- Cross-platform clipboard support (macOS, Linux, Android/Termux, Windows)
- Usage: `!y` then select responses to copy

## Platform Compatibility

Ch supports multiple AI platforms with seamless switching:

| Platform  | Models                     | Environment Variable |
| --------- | -------------------------- | -------------------- |
| OpenAI    | GPT-4o, GPT-4o-mini, etc.  | `OPENAI_API_KEY`     |
| Groq      | Llama3, Mixtral, etc.      | `GROQ_API_KEY`       |
| DeepSeek  | DeepSeek-Chat, etc.        | `DEEP_SEEK_API_KEY`  |
| Anthropic | Claude-3.5, etc.           | `ANTHROPIC_API_KEY`  |
| xAI       | Grok models                | `XAI_API_KEY`        |
| Together  | Llama3, Mixtral, etc.      | `TOGETHER_API_KEY`   |
| Google    | Gemini models              | `GEMINI_API_KEY`     |
| Mistral   | Mistral-tiny, small, etc.  | `MISTRAL_API_KEY`    |
| Ollama    | Local models (Llama3, etc) | (none)               |

Switch platforms during conversation:

```bash
!p groq
!p anthropic
!m gpt-4o
```

## Development

### Prerequisites

- Go 1.21 or higher
- [fzf](https://github.com/junegunn/fzf) for interactive selections
- [curl](https://curl.se/) for web content fetching
- [lynx](https://lynx.browser.org/) for text extraction from HTML
- [yt-dlp](https://github.com/yt-dlp/yt-dlp) for YouTube video scraping
- [ddgr](https://github.com/jarun/ddgr) for DuckDuckGo web search
- Clipboard utilities (auto-detected): pbcopy, xclip, xsel, wl-copy, termux-clipboard-set
- [Helix editor](https://helix-editor.com/) (optional but recommended for enhanced text editing)

### Build from Source

```bash
git clone https://github.com/MehmetMHY/ch.git

cd ch

# build locally without installing
./install.sh -b
```

### Build Options

```bash
# using the install script (local build options)
./install.sh -b     # build locally without installing
./install.sh -r -b  # refresh/update all dependencies and build
./install.sh -h     # show help with all options

# using Make directly
make install   # install to $GOPATH/bin
make clean     # clean build artifacts
make test      # run tests
make lint      # run linter
make fmt       # format code
make dev       # build and run in dev mode
```

## Contributing

Contributions are welcome! Here's how to get started:

1. **Report Issues**: [Open an issue](https://github.com/MehmetMHY/ch/issues) for bugs or feature requests
2. **Submit Pull Requests**: Fork, make changes, and submit a PR
3. **Improve Documentation**: Help enhance README, examples, or guides

### Development Setup

```bash
git clone https://github.com/MehmetMHY/ch.git

cd ch

# refresh dependencies and build
./install.sh -r -b

make dev
```

### Code Standards

- Follow existing Go conventions
- Run `make fmt` and `make lint` before submitting
- Test your changes thoroughly
- Update documentation as needed
- To add new slow models, update patterns in `internal/platform/platform.go`

## Uninstall

Ch can be uninstalled using the install script's uninstall option (see [Installation](#installation) section) or manually:

```bash
# manual uninstall for Unix-based systems
sudo rm -f /usr/local/bin/ch
rm -rf ~/.ch

# manual uninstall for Android/Termux systems
rm -f $PREFIX/bin/ch
rm -rf ~/.ch
```

## License

Ch is licensed under the MIT License. See [LICENSE](./LICENSE) for details.
