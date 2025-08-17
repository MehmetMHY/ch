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

**Ch** is a lightweight, GoLang-based version of the popular [Cha](https://github.com/MehmetMHY/cha/) CLI tool. Built from the ground up in Go, Ch is over 10x faster at startup compared to Cha and delivers significantly faster performance for complex processes like codedump. While Cha offers more advanced and powerful features with a larger codebase, Ch prioritizes speed and efficiency, making it ideal for developers who need rapid AI interaction with minimal overhead.

## Vision

Ch follows the Unix philosophy of doing one thing well. It provides direct terminal access to powerful AI models with minimal overhead, transparent operations, and explicit user control. No hidden automation, no surprise file modifications—just fast, reliable AI interaction that integrates seamlessly into your development workflow.

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

- **High Performance**: 2.55x faster than the original Python implementation
- **Multi-Platform Support**: OpenAI, Groq, DeepSeek, Anthropic, XAI, and Ollama
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
- **Shell Session Recording**: Record terminal sessions and provide them as context to the model.
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

- Checks for Go 1.21+ and dependencies (fzf)
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
export DEEPSEEK_API_KEY="your-deepseek-key"
export ANTHROPIC_API_KEY="your-anthropic-key"
export XAI_API_KEY="your-xai-key"
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
- **`!l`** - Load files (text, PDF, DOCX, XLSX, CSV) and directories
- **`!d`** - Generate code dump
- **`!e`** - Export selected chat entries
- **`!x`** - Record shell session or run command (`!x ls` streams output live)
- **`\`** - Multi-line input mode
- **`|`** - View and display chat history

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

## Web Scraping

Ch includes a complementary web scraping tool that provides focused content extraction without adding complexity to the main CLI. The scraper handles YouTube videos and general web pages, and includes interactive search functionality via DuckDuckGo.

**Installation:**

```bash
cd scraper/
./install.sh
cd -
```

**Update dependencies:**

```bash
cd scraper/
./install.sh -r    # or --update
cd -
```

**Uninstall:**

```bash
cd scraper/
./install.sh -u
cd -
```

**Usage with Ch:**

```bash
# scrape specific URLs
ch
!x scrape https://example.com

# search and interactively select URLs to scrape
ch
!x scrape -s "machine learning basics"
```

The scraper extracts clean, structured content from web pages and YouTube videos (including metadata and subtitles). The search feature uses DuckDuckGo to find relevant content and lets you interactively select which URLs to scrape using fzf. All scraped content becomes available as context in your AI conversation through Ch's shell session recording feature.

For detailed usage instructions, see the [scraper README](./scraper/README.md).

## Platform Compatibility

Ch supports multiple AI platforms with seamless switching:

| Platform  | Models                     | Environment Variable |
| --------- | -------------------------- | -------------------- |
| OpenAI    | GPT-4o, GPT-4o-mini, etc.  | `OPENAI_API_KEY`     |
| Groq      | Llama3, Mixtral, etc.      | `GROQ_API_KEY`       |
| DeepSeek  | DeepSeek-Chat, etc.        | `DEEPSEEK_API_KEY`   |
| Anthropic | Claude-3.5, etc.           | `ANTHROPIC_API_KEY`  |
| xAI       | Grok models                | `XAI_API_KEY`        |
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
./install.sh -d -b  # update dependencies and build
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

# update dependencies and build
./install.sh -d -b

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
