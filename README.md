# Cha-Go

A Go chat client supporting multiple AI platforms with web search integration.

## Quick Start

1. **Build**

   ```bash
   go mod tidy && chmod +x build.sh && ./build.sh
   ```

2. **Set API keys**

   ```bash
   export OPENAI_API_KEY="your-key"
   export GROQ_API_KEY="your-key"
   # Add other platforms as needed
   ```

3. **Run**
   ```bash
   ./cha-go          # Interactive mode (type 'help' to see all options)
   ./cha-go "prompt" # Direct query
   ```

## Key Features

- **Multi-platform support**: OpenAI, Groq, DeepSeek, Anthropic, XAI
- **Web search**: SearXNG integration with IEEE citations
- **Interactive commands**: `!p` (platform), `!sm` (model), `!w` (web search)

## Requirements

- Go 1.21+
- [fzf](https://github.com/junegunn/fzf) - `brew install fzf`
- [Docker](https://www.docker.com/get-started/)

## Web Search Setup

```bash
cd sxng && python3 run.py
```

Then use `!w <query>` in chat for web-enhanced responses.
