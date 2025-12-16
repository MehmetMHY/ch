<div align="center">
  <img src="./docs/logo.png" width="200">
</div>

# Ch

<a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License: MIT"></a> <a href="https://golang.org/"><img src="https://img.shields.io/badge/go-1.21+-blue.svg" alt="Go 1.21+"></a> <a href="https://github.com/MehmetMHY/ch/graphs/contributors"><img src="https://img.shields.io/github/contributors/MehmetMHY/ch" alt="Contributors"></a>

<p align="left">
  <a href="https://www.youtube.com/watch?v=AH0xG1iStf4">
    <img src="https://github.com/user-attachments/assets/4a0df463-089c-4b91-956d-3ab992874307" alt="Demo GIF">
  </a><br>
  <em>Check out the <a href="https://www.youtube.com/watch?v=AH0xG1iStf4">full demo on YouTube</a></em>
</p>

## Table of Contents

- [Overview](#overview)
- [Vision](#vision)
- [Quick Start](#quick-start)
- [Features](#features)
- [Installation](#installation)
- [Configuration](#configuration)
  - [API Keys](#api-keys)
  - [Default Settings](#default-settings)
  - [Config File](#config-file)
  - [Local & Open-Source Setup](#local--open-source-setup)
- [Usage](#usage)
  - [Basic Usage](#basic-usage)
  - [Interactive Commands](#interactive-commands)
  - [Advanced Features](#advanced-features)
  - [Web Content Interaction](#web-content-interaction)
- [Platform Compatibility](#platform-compatibility)
- [Website](#website)
- [Development](#development)
  - [Prerequisites](#prerequisites)
  - [Build from Source](#build-from-source)
  - [Build Options](#build-options)
- [Contributing](#contributing)
  - [Development Setup](#development-setup)
  - [Code Standards](#code-standards)
- [Uninstall](#uninstall)
  - [Clean Temporary Files](#clean-temporary-files)
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
- **Multi-Platform Support**: OpenAI, Groq, DeepSeek, Anthropic, XAI, Together, Google Gemini, Mistral AI, Amazon Bedrock, and Ollama
- **Multi-Region Support**: Switch between regional endpoints for platforms like Amazon Bedrock (22 AWS regions)
- **Interactive & Direct Modes**: Chat interactively or run single queries
- **Unix Piping**: Pipe any command output or file content directly to Ch
- **Seamless Pipe Output**: Automatically suppresses colors and UI elements when output is piped, perfect for shell pipelines and automation
- **Smart File Handling**: Load text files, PDFs, Word docs (DOCX/ODT/RTF), spreadsheets (XLSX/CSV), images (with OCR text extraction), and directories
- **Advanced Export**: Interactive chat export with fzf selection and editor integration
- **Code Block Export**: Extract and save markdown code blocks with proper file extensions
- **Session State Viewer**: Check current session details like model, platform, and token usage
- **Token Counting**: Estimate token usage for files with model-aware tokenization
- **Text Editor Integration**: Use your preferred editor for complex prompts
- **Dynamic Switching**: Change models and platforms mid-conversation
- **Chat Backtracking**: Revert to any point in conversation history
- **Session Continuation**: Automatically save and restore sessions to continue conversations later
- **Session History Search**: Search and load any previous session from history with fuzzy or exact matching
- **Code Dump**: Package entire directories for AI analysis (text and document files only)
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
# safe uninstall with confirmation prompt (recommended)
./install.sh --safe-uninstall

# or uninstall without confirmation
./install.sh --uninstall
```

The installer automatically:

- Checks for Go 1.21+ and dependencies (fzf, yt-dlp, tesseract).
- Installs missing dependencies via system package managers (apt, brew, pkg, etc.).
- Builds and installs Ch to `~/.ch/bin/ch` with temporary files in `~/.ch/tmp/`.
- Attempts to create a global symlink at `/usr/local/bin/ch` (or `$PREFIX/bin/ch` on Android/Termux).
- If the symlink creation fails due to permissions, it will automatically install to `~/.ch/bin` and provide instructions to add it to your `PATH`.
- Warns you if Tesseract OCR is not installed, as it is required for image-to-text extraction.

## Configuration

### API Keys

Set up API keys for your chosen platforms. `OPENAI_API_KEY` is required for core functionality, and `BRAVE_API_KEY` is required for the web search feature.

#### Important Note on API Keys

By default, Ch uses the `openai` platform. If you run `ch` without setting the `OPENAI_API_KEY`, you will see an error. Here’s how to get started:

1.  **Set the API Key**: If you want to use OpenAI, set the environment variable:
    ```bash
    export OPENAI_API_KEY="your-openai-key"
    ```
2.  **Switch Platforms**: Use a different platform that you have configured. For example, to use Groq:
    ```bash
    ch -p groq "Hello"
    ```
3.  **Use a Local Model**: For a completely free and offline experience, use [Ollama](https://ollama.com/):
    ```bash
    ch -p ollama "Hello"
    ```

```bash
# required
export OPENAI_API_KEY="your-openai-key"
export BRAVE_API_KEY="your-brave-api-key" # for web search

# optional
export GROQ_API_KEY="your-groq-key"
export DEEP_SEEK_API_KEY="your-deepseek-key"
export ANTHROPIC_API_KEY="your-anthropic-key"
export XAI_API_KEY="your-xai-key"
export TOGETHER_API_KEY="your-together-key"
export GEMINI_API_KEY="your-gemini-key"
export MISTRAL_API_KEY="your-mistral-key"
export AWS_BEDROCK_API_KEY="your-bedrock-key"
```

You can find links to obtain API keys below:

| Platform       | Get API Key                                        |
| -------------- | -------------------------------------------------- |
| OpenAI         | https://openai.com/api/                            |
| Brave Search   | https://brave.com/search/api/                      |
| Google Gemini  | https://ai.google.dev/gemini-api/docs/api-key      |
| xAI            | https://x.ai/api                                   |
| Groq           | https://console.groq.com/keys                      |
| Mistral AI     | https://docs.mistral.ai/getting-started/quickstart |
| Anthropic      | https://console.anthropic.com/                     |
| Together AI    | https://docs.together.ai/docs/quickstart           |
| DeepSeek       | https://api-docs.deepseek.com/                     |
| Amazon Bedrock | https://aws.amazon.com/bedrock/                    |

### Default Settings

Customize default platform and model via environment variables:

```bash
# default: openai
export CH_DEFAULT_PLATFORM="groq"

# default: gpt-4.1-mini
export CH_DEFAULT_MODEL="llama3-8b-8192"
```

### Config File

For persistent configuration, create `~/.ch/config.json` to override default settings without needing environment variables:

```json
{
  "default_model": "grok-4-fast-non-reasoning",
  "current_platform": "xai",
  "preferred_editor": "vim",
  "show_search_results": true,
  "num_search_results": 10,
  "search_country": "us",
  "search_lang": "en",
  "system_prompt": "You are a helpful assistant."
}
```

**Available config options:**

- `default_model` - Set default model (automatically sets current_model if not specified)
- `current_model` - Set current active model
- `current_platform` - Set default platform
- `current_base_url` - Set default base URL/region for multi-region platforms like Amazon Bedrock
- `preferred_editor` - Set preferred text editor (default: "vim")
- `show_search_results` - Show/hide web search results (default: false)
- `num_search_results` - Number of search results to display (default: 5)
- `search_country` - Set the country for web searches (default: "us")
- `search_lang` - Set the language for web searches (default: "en")
- `system_prompt` - Customize the system prompt
- `enable_session_save` - Enable/disable automatic session saving for continuation (default: true)
- `save_all_sessions` - Save all sessions with timestamps instead of overwriting the latest (default: false). When enabled, each session gets a unique timestamped file; when disabled, only the latest session is kept
- `shallow_load_dirs` - Directories to load with only 1-level depth for `!l` and `!e` operations (default: major system directories like `/`, `/home/`, `/usr/`, `$HOME`, etc.). Set to `[]` to disable.
- Plus all other configuration options using snake_case JSON field names

For a complete list of all configuration options and their defaults, see [internal/config/config.go](./internal/config/config.go). But note that the config file takes precedence over environment variables and provides a convenient way to customize Ch without setting environment variables for each session.

### Local & Open-Source Setup

Ch supports local models via [Ollama](https://ollama.com/), allowing you to run it without relying on third-party services. This provides a completely private, open-source, and offline-capable environment.

1.  **Install Ollama**: Follow the official instructions at [ollama.com](https://ollama.com).
2.  **Pull a model**: `ollama pull llama3`

3.  **Run Ch with Ollama**: `ch -p ollama "What is the capital of France?"`

Since **Ollama** runs locally, no API key is required.

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

# platform and model together
ch -o openai|gpt-4o "Create a REST API in Python"

# export code blocks to files
ch -e "Write a Python script to sort a list"

# load and display file content
ch -l document.pdf
ch -l document.docx  # or .odt, .rtf
ch -l spreadsheet.xlsx
ch -l screenshot.png

# scrape web content
ch -l https://example.com
ch -l https://youtube.com/watch?v=example

# count tokens in files
ch -t ./README.md
ch -m "gpt-4" -t ./main.go

# piping support (colors/UI automatically suppressed)
cat main.py | ch "What does this code do?"
echo "hello world" | ch "Translate to Spanish"
ls -la | ch "Summarize this directory"

# perfect for shell pipelines and automation
ch "list 5 fruits" | grep apple
ch "explain golang" > output.txt
ch -w "golang features" | head -10

# session continuation - automatically saves and restores conversations
ch -c                              # continue last session interactively
ch -c "follow up question"         # continue with a new query
ch -a                              # fuzzy search and load a previous session
ch -a exact                        # exact match search for previous sessions
ch --clear                         # clear all temporary files and sessions
```

### Interactive Commands

When in interactive mode (`ch`), use these commands:

- **`!q`** - exit interface
- **`!h`** - help page
- **`!c`** - clear chat history
- **`!b`** - backtrack messages
- **`!t [buff]`** - text editor mode
- **`\`** - multi-line mode (exit with `\`)
- **`!m`** - switch models
- **`!o`** - select from all models
- **`!p`** - switch platforms
- **`!l [dir]`** - load files/dirs
- **`!a [exact]`** - search and load sessions
- **`!x`** - record shell session
- **`!s [url]`** - scrape URL(s) or from history
- **`!w [query]`** - web search or from history
- **`!d`** - generate codedump
- **`!e`** - export chat(s)
- **`!y`** - add to clipboard
- **`ctrl+c`** - clear prompt input
- **`ctrl+d`** - exit completely

### Advanced Features

**Code Export (`-e` flag):**

- Automatically detects programming languages
- Saves with proper file extensions
- Supports 25+ languages and file types

**Interactive Export (`!e`):**

Offers two modes for exporting chat history:

1.  **auto export**: Automatically extracts all code blocks from your entire chat history. It then lets you save each snippet individually, intelligently suggesting file names and extensions based on the code's language and content. It presents a single, prioritized list of suggested new names and existing files (marked with `[w]` for overwrite).
2.  **manual export**: Allows you to select specific chat entries, which are then combined into a single file for you to edit and save manually. This mode also benefits from the smart file-saving interface.

**URL Scraping (`!s` and `-l` with URLs):**

- Supports regular web pages and YouTube videos
- Extracts clean text content from web pages using a built-in parser
- YouTube videos include metadata and subtitle extraction via yt-dlp
- Multiple URL support: `!s https://site1.com https://site2.com`
- Interactive URL selection: When called without arguments (`!s`), scans chat history for all URLs, removes duplicates, and presents them via fzf for multi-selection with tab key
- Integrated with file loading: `ch -l https://example.com`

**Web Search (`!w`):**

- Built-in Brave Search integration via the Brave Search API
- Requires `BRAVE_API_KEY` to be set in your environment variables
- Usage: `!w "search query"` or `!w` to select a sentence from chat history
- Results are automatically added to conversation context
- No need for external tools, but requires an API key

**Clipboard Copy (`!y`):**

- Select one or more AI responses with fzf
- Edit content in your preferred editor before copying
- Cross-platform clipboard support (macOS, Linux, Android/Termux, Windows)
- Usage: `!y` then select responses to copy

### Web Content Interaction

The `-s` and `-w` flags in the terminal CLI are used for web content interaction:

#### `-s` flag (Scrape URL)

- Usage: `ch -s <URL>`
- Function: Scrapes content from the specified URL.
- Supports scraping normal web pages and YouTube videos.
- For normal web pages, it fetches and extracts clean text content from the HTML.
- For YouTube URLs, it uses `yt-dlp` to extract metadata and subtitles.
- The scraped content is printed directly to the terminal.

#### `-w` flag (Web Search)

- Usage: `ch -w <search query>`
- Function: Performs a web search using the Brave Search API.
- Requires `BRAVE_API_KEY` environment variable to be set.
- Fetches search results from Brave Search.
- Prints the formatted search results (title, URL, description) to the terminal.

Both commands help in integrating external web content and search results into CLI workflow with Ch.

## Platform Compatibility

Ch supports multiple AI platforms with seamless switching:

| Platform       | Models                      | Environment Variable  | Regions/Endpoints |
| -------------- | --------------------------- | --------------------- | ----------------- |
| OpenAI         | GPT-4o, GPT-4o-mini, etc.   | `OPENAI_API_KEY`      | 1                 |
| Groq           | Llama3, Mixtral, etc.       | `GROQ_API_KEY`        | 1                 |
| DeepSeek       | DeepSeek-Chat, etc.         | `DEEP_SEEK_API_KEY`   | 1                 |
| Anthropic      | Claude-3.5, etc.            | `ANTHROPIC_API_KEY`   | 1                 |
| xAI            | Grok models                 | `XAI_API_KEY`         | 1                 |
| Together       | Llama3, Mixtral, etc.       | `TOGETHER_API_KEY`    | 1                 |
| Google         | Gemini models               | `GEMINI_API_KEY`      | 1                 |
| Mistral        | Mistral-tiny, small, etc.   | `MISTRAL_API_KEY`     | 1                 |
| Amazon Bedrock | Claude, Llama, Mistral, etc | `AWS_BEDROCK_API_KEY` | 22                |
| Ollama         | Local models (Llama3, etc)  | (none)                | 1                 |

Switch platforms during conversation:

```bash
!p groq
!p anthropic
!m gpt-4o
```

**Multi-Region Platforms:**

Some platforms like Amazon Bedrock support multiple regions. When switching to a multi-region platform, you'll be prompted to select a region before choosing a model:

```bash
!p amazon
# Prompts: region: (select from 22 AWS regions)
# Prompts: model: (select from available models in that region)
```

Supported AWS Bedrock regions: US East (N. Virginia, Ohio), US West (Oregon), Asia Pacific (Tokyo, Seoul, Osaka, Mumbai, Hyderabad, Singapore, Sydney), Canada (Central), Europe (Frankfurt, Ireland, London, Milan, Paris, Spain, Stockholm, Zurich), South America (São Paulo), AWS GovCloud (US-East, US-West), and FIPS endpoints.

## Website

The project website is hosted on **[GitHub Pages](https://docs.github.com/en/pages)** right [HERE](https://mehmetmhy.github.io/ch/). See the [`docs/`](./docs/README.md) sub-directory for the website source and build script.

## Development

### Prerequisites

- Go 1.21 or higher
- [fzf](https://github.com/junegunn/fzf) for interactive selections
- [yt-dlp](https://github.com/yt-dlp/yt-dlp) for [YouTube](https://www.youtube.com/) video scraping
- [Tesseract OCR](https://github.com/tesseract-ocr/tesseract) **(optional)** for image-to-text extraction from images. The installer will warn you if it's missing.
- `BRAVE_API_KEY` for web search (see [API Keys](#api-keys))
- **Clipboard utils (auto-detected)**: [pbcopy](https://ss64.com/mac/pbcopy.html), [xclip](https://github.com/astrand/xclip), [xsel](https://github.com/kfish/xsel), [wl-copy](https://man.archlinux.org/man/wl-copy.1.en), [termux-clipboard-set](https://wiki.termux.com/wiki/Termux-clipboard-set)
- [Vim](https://www.vim.org/) but [Helix IDE](https://helix-editor.com/) is recommended

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
./install.sh -v     # update version in Makefile interactively
./install.sh -h     # show help with all options

# using Make directly
make install  # install to $GOPATH/bin
make clean    # clean build artifacts
make test     # run tests
make lint     # run linter
make fmt      # format code
make dev      # build and run in dev mode
```

### Version Management

Update the project version interactively:

```bash
./install.sh -v
```

This will:

- Display the current version from Makefile
- Offer semantic version bump options (patch, minor, major)
- Allow custom version input
- Update the VERSION in Makefile automatically

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

Use `--safe-uninstall` for a confirmation prompt before deletion (recommended). The `--uninstall` flag deletes immediately without confirmation.

```bash
# safe uninstall with confirmation prompt (recommended)
./install.sh --safe-uninstall

# or uninstall without confirmation
./install.sh --uninstall
```

Manual uninstall:

```bash
# manual uninstall for Unix-based systems
sudo rm -f /usr/local/bin/ch
rm -rf ~/.ch

# manual uninstall for Android/Termux systems
rm -f $PREFIX/bin/ch
rm -rf ~/.ch
```

### Clean Temporary Files

If you want to safely remove all Ch temporary files without uninstalling the application:

```bash
[ -d "${HOME}/.ch/tmp/" ] && rm -rf "${HOME}/.ch/tmp/"
```

This is useful for reclaiming disk space if temporary files from shell sessions, file loads, or other operations have accumulated.

## License

Ch is licensed under the **MIT License**. See [LICENSE](./LICENSE) for details.
